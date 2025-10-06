package watchdog

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/prometheus/client_golang/prometheus"
	"sync/atomic"
	"time"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("watchdog")
}

type BrokerStatus struct {
	BrokerType model.MessageBrokerType
	Status     StatusDetailed
}

type BrokerInstancesMonitor struct {
	rabbitProvider      RabbitInstanceProvider
	kafkaProvider       KafkaInstanceProvider
	healthCheckInterval time.Duration

	statusCache            atomic.Pointer[map[string]BrokerStatus]
	needBrokerListsRefresh atomic.Int32
	kafkaBrokersCache      atomic.Pointer[[]model.KafkaInstance]
	rabbitBrokersCache     atomic.Pointer[[]model.RabbitInstance]

	brokerMetrics map[string]prometheus.Gauge
}

//go:generate mockgen -source=broker_instance_monitor.go -destination=mock/broker_instance_monitor.go
type KafkaInstanceProvider interface {
	CheckHealth(context.Context, *model.KafkaInstance) error
	GetKafkaInstances(ctx context.Context) (*[]model.KafkaInstance, error)
	AddInstanceUpdateCallback(func(ctx context.Context, instance *model.KafkaInstance))
}

type RabbitInstanceProvider interface {
	CheckHealth(context.Context, *model.RabbitInstance) error
	GetRabbitInstances(ctx context.Context) (*[]model.RabbitInstance, error)
	AddInstanceUpdateCallback(func(ctx context.Context, instance *model.RabbitInstance))
}

func NewBrokerInstancesMonitor(kafkaProvider KafkaInstanceProvider, rabbitProvider RabbitInstanceProvider, healthCheckInterval time.Duration) *BrokerInstancesMonitor {
	inst := &BrokerInstancesMonitor{kafkaProvider: kafkaProvider, rabbitProvider: rabbitProvider, healthCheckInterval: healthCheckInterval}
	kafkaProvider.AddInstanceUpdateCallback(inst.enqueueKafkaInstanceListUpdate)
	rabbitProvider.AddInstanceUpdateCallback(inst.enqueueRabbitInstanceListUpdate)
	inst.needBrokerListsRefresh.Add(1) // force update instance list on Start()
	return inst
}

func (bsm *BrokerInstancesMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(bsm.healthCheckInterval)
	bsm.updateStatusCache(ctx)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				bsm.updateStatusCache(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

func (bsm *BrokerInstancesMonitor) refreshBrokerList(ctx context.Context) error {
	log.InfoC(ctx, "Reload kafka and rabbit instances health cache")
	kafkaInstances, err := bsm.kafkaProvider.GetKafkaInstances(ctx)
	if err != nil {
		return utils.LogError(log, ctx, "error getting list of kafka instances: %w", err)
	}
	rabbitInstances, err := bsm.rabbitProvider.GetRabbitInstances(ctx)
	if err != nil {
		return utils.LogError(log, ctx, "error getting list of rabbitmq instances: %w", err)
	}

	bsm.kafkaBrokersCache.Store(kafkaInstances)
	bsm.rabbitBrokersCache.Store(rabbitInstances)

	// update metric collector list
	for _, m := range bsm.brokerMetrics {
		prometheus.Unregister(m)
	}
	bsm.brokerMetrics = make(map[string]prometheus.Gauge)
	for _, instance := range *kafkaInstances {
		bsm.brokerMetrics[string(instance.BrokerType())+instance.GetId()] = registerMetricCollector(ctx, instance.GetId(), instance.BrokerType())
	}
	for _, instance := range *rabbitInstances {
		bsm.brokerMetrics[string(instance.BrokerType())+instance.GetId()] = registerMetricCollector(ctx, instance.GetId(), instance.BrokerType())
	}

	return nil
}

func registerMetricCollector(ctx context.Context, brokerId string, brokerType model.MessageBrokerType) prometheus.Gauge {
	gauge := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "maas",
		Subsystem: "health",
		Name:      "broker_status",
		Help:      "indicates health status of registered message broker: UNKNOWN=0, UP=10, WARNING=20, PROBLEM=30",
		ConstLabels: map[string]string{
			"broker_id":   brokerId,
			"broker_type": string(brokerType),
		},
	})

	if err := prometheus.Register(gauge); err != nil {
		_ = utils.LogError(log, ctx, "error register health gauge for %v broker with id: %v: %w", brokerType, brokerId, err)
	}
	return gauge
}

func (bsm *BrokerInstancesMonitor) All() []BrokerStatus {
	ref := bsm.statusCache.Load()
	if ref == nil {
		return nil
	}

	var result []BrokerStatus
	for _, s := range *ref {
		result = append(result, s)
	}
	return result
}

// this code must be efficient, because it will be called in audit service to produce output
func (bsm *BrokerInstancesMonitor) GetStatus(instanceId string) Status {
	ref := bsm.statusCache.Load()
	if ref == nil {
		return UNKNOWN
	}

	if data, ok := (*ref)[instanceId]; ok {
		return data.Status.Status
	} else {
		return UNKNOWN
	}
}

func (bsm *BrokerInstancesMonitor) updateStatusCache(ctx context.Context) {
	if bsm.needBrokerListsRefresh.Load() > 0 {
		if err := bsm.refreshBrokerList(ctx); err == nil {
			bsm.needBrokerListsRefresh.Add(-1)
		} else {
			log.ErrorC(ctx, "Error update brokers instance list. Health check result will be inconsistent: %v", err.Error())
		}
	}

	statuses := make(map[string]BrokerStatus)
	for _, instance := range *bsm.rabbitBrokersCache.Load() {
		health := bsm.getHealth(&instance, bsm.rabbitProvider.CheckHealth(ctx, &instance))
		statuses[instance.GetId()] = health
		if m, ok := bsm.brokerMetrics[string(instance.BrokerType())+instance.GetId()]; ok {
			m.Set(float64(health.Status.Status.Int()))
		}
	}
	for _, instance := range *bsm.kafkaBrokersCache.Load() {
		health := bsm.getHealth(&instance, bsm.kafkaProvider.CheckHealth(ctx, &instance))
		statuses[instance.GetId()] = health
		if m, ok := bsm.brokerMetrics[string(instance.BrokerType())+instance.GetId()]; ok {
			m.Set(float64(health.Status.Status.Int()))
		}
	}

	bsm.statusCache.Store(&statuses)
}

func (bsm *BrokerInstancesMonitor) getHealth(instance model.MessageBrokerInstance, error error) BrokerStatus {
	var status StatusDetailed
	if error != nil {
		status = StatusDetailed{PROBLEM, fmt.Sprintf("%s: %s", instance.GetId(), error.Error())}
	} else {
		status = StatusDetailed{Status: UP}
	}
	return BrokerStatus{instance.BrokerType(), status}
}

func (bsm *BrokerInstancesMonitor) enqueueKafkaInstanceListUpdate(_ context.Context, _ *model.KafkaInstance) {
	bsm.needBrokerListsRefresh.Add(+1)
}

func (bsm *BrokerInstancesMonitor) enqueueRabbitInstanceListUpdate(_ context.Context, _ *model.RabbitInstance) {
	bsm.needBrokerListsRefresh.Add(+1)
}
