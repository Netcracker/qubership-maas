// Package discrepancy exports prometheus metrics describing the difference between
// entities registered in the maas database and entities actually existing on the brokers.
//
// For every registered broker instance three numbers are reported:
//   - registered: entities maas knows about in its own database
//   - lost:       registered in maas, but missing on the broker
//   - ghost:      present on the broker, but not registered in maas
//
// Ghost detection is limited to entities following the maas naming convention
// (see maasEntityPrefix), otherwise foreign topics/vhosts living on a shared broker
// would be reported as ghosts. Entities created by maas under a custom name template
// are therefore not counted as ghosts - the metric under-reports rather than produces
// false positives.
package discrepancy

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/IBM/sarama"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// entities created by maas are named "maas.<namespace>..."
	maasEntityPrefix = "maas."

	// same values as the maas_health_broker_status metric, so both can be filtered alike
	brokerTypeKafka  = string(model.Kafka)
	brokerTypeRabbit = string(model.RabbitMQ)

	defaultCollectInterval = 5 * time.Minute
)

var log logging.Logger

func init() {
	log = logging.GetLogger("discrepancy-metrics")
}

//go:generate mockgen -source=collector.go -destination=mock/collector.go -package=mock
type KafkaInstanceProvider interface {
	GetKafkaInstances(ctx context.Context) (*[]model.KafkaInstance, error)
}

type KafkaTopicProvider interface {
	SearchTopicsInDB(ctx context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistration, error)
}

type KafkaBrokerLister interface {
	GetListTopics(ctx context.Context, instance *model.KafkaInstance) (map[string]sarama.TopicDetail, error)
}

type RabbitInstanceProvider interface {
	GetRabbitInstances(ctx context.Context) (*[]model.RabbitInstance, error)
}

type RabbitVhostProvider interface {
	FindVhostWithSearchForm(ctx context.Context, searchForm *model.SearchForm) ([]model.VHostRegistration, error)
}

// RabbitHelperFactory produces an instance scoped rabbit helper. Only instance level
// calls (GetAllVhosts) are allowed on the result, it is not bound to any vhost.
type RabbitHelperFactory func(instance model.RabbitInstance) helper.RabbitHelper

func DefaultRabbitHelperFactory(instance model.RabbitInstance) helper.RabbitHelper {
	return helper.NewRabbitHelperWithHttpHelper(instance, model.VHostRegistration{}, helper.NewHttpHelper())
}

// entityCounts is a discrepancy snapshot of a single broker instance
type entityCounts struct {
	registered int
	lost       int
	ghost      int
	available  bool
}

type instanceKey struct {
	brokerType string
	instanceId string
}

type MetricCollector struct {
	kafkaInstances KafkaInstanceProvider
	kafkaTopics    KafkaTopicProvider
	kafkaBroker    KafkaBrokerLister

	rabbitInstances RabbitInstanceProvider
	rabbitVhosts    RabbitVhostProvider
	rabbitHelperOf  RabbitHelperFactory

	collectInterval time.Duration

	// last successfully calculated numbers per instance. Kept so that a temporarily
	// unreachable broker doesn't report all its entities as lost.
	lastKnown map[instanceKey]entityCounts

	registeredMetric *prometheus.GaugeVec
	lostMetric       *prometheus.GaugeVec
	ghostMetric      *prometheus.GaugeVec
	availableMetric  *prometheus.GaugeVec
}

func NewMetricCollector(
	kafkaInstances KafkaInstanceProvider,
	kafkaTopics KafkaTopicProvider,
	kafkaBroker KafkaBrokerLister,
	rabbitInstances RabbitInstanceProvider,
	rabbitVhosts RabbitVhostProvider,
	rabbitHelperOf RabbitHelperFactory,
	collectInterval time.Duration,
) *MetricCollector {
	if collectInterval <= 0 {
		collectInterval = defaultCollectInterval
	}
	return &MetricCollector{
		kafkaInstances:  kafkaInstances,
		kafkaTopics:     kafkaTopics,
		kafkaBroker:     kafkaBroker,
		rabbitInstances: rabbitInstances,
		rabbitVhosts:    rabbitVhosts,
		rabbitHelperOf:  rabbitHelperOf,
		collectInterval: collectInterval,
		lastKnown:       make(map[instanceKey]entityCounts),

		registeredMetric: registerGaugeVec("registered_entities",
			"number of entities registered in maas database"),
		lostMetric: registerGaugeVec("lost_entities",
			"number of entities registered in maas database, but missing on the broker"),
		ghostMetric: registerGaugeVec("ghost_entities",
			"number of maas named entities existing on the broker, but not registered in maas database"),
		availableMetric: registerGaugeVec("broker_reachable",
			"1 if the last discrepancy calculation reached the broker, 0 if numbers are stale"),
	}
}

func registerGaugeVec(name string, help string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "maas",
		Subsystem: "discrepancy",
		Name:      name,
		Help:      help,
	}, []string{"broker_type", "broker_id"})

	if err := prometheus.Register(gaugeVec); err != nil {
		var alreadyRegistered prometheus.AlreadyRegisteredError
		if errors.As(err, &alreadyRegistered) {
			return alreadyRegistered.ExistingCollector.(*prometheus.GaugeVec)
		}
		log.Error("error register discrepancy gauge '%v': %v", name, err)
	}
	return gaugeVec
}

func (c *MetricCollector) Start(ctx context.Context) {
	ticker := time.NewTicker(c.collectInterval)
	go func() {
		defer ticker.Stop()
		c.Collect(ctx)
		for {
			select {
			case <-ticker.C:
				c.Collect(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
}

// Collect recalculates discrepancy for all registered broker instances and publishes
// the result to prometheus. Instances that failed to respond keep their previous numbers
// and are marked as unreachable.
func (c *MetricCollector) Collect(ctx context.Context) {
	current := make(map[instanceKey]entityCounts)

	c.collectKafka(ctx, current)
	c.collectRabbit(ctx, current)

	// instances removed from maas must not leave stale series behind
	c.registeredMetric.Reset()
	c.lostMetric.Reset()
	c.ghostMetric.Reset()
	c.availableMetric.Reset()

	c.lastKnown = current
	for key, counts := range current {
		labels := prometheus.Labels{"broker_type": key.brokerType, "broker_id": key.instanceId}
		c.registeredMetric.With(labels).Set(float64(counts.registered))
		c.lostMetric.With(labels).Set(float64(counts.lost))
		c.ghostMetric.With(labels).Set(float64(counts.ghost))
		c.availableMetric.With(labels).Set(boolToFloat(counts.available))
	}
}

func (c *MetricCollector) collectKafka(ctx context.Context, result map[instanceKey]entityCounts) {
	instances, err := c.kafkaInstances.GetKafkaInstances(ctx)
	if err != nil {
		log.ErrorC(ctx, "error getting list of kafka instances, kafka discrepancy metrics will not be updated: %v", err)
		return
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeKafka, instance.GetId()}

		topicsInDb, err := c.kafkaTopics.SearchTopicsInDB(ctx, &model.TopicSearchRequest{Instance: instance.GetId()})
		if err != nil {
			log.ErrorC(ctx, "error getting topics of kafka instance '%v' from db, keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleCounts(key)
			continue
		}
		registered := make(map[string]bool, len(topicsInDb))
		for _, topic := range topicsInDb {
			registered[topic.Topic] = true
		}

		topicsOnBroker, err := c.kafkaBroker.GetListTopics(ctx, &instance)
		if err != nil {
			log.WarnC(ctx, "error getting list of topics from kafka instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			counts := c.staleCounts(key)
			counts.registered = len(registered)
			result[key] = counts
			continue
		}

		existing := make(map[string]bool, len(topicsOnBroker))
		for name := range topicsOnBroker {
			existing[name] = true
		}

		result[key] = compare(registered, existing)
	}
}

func (c *MetricCollector) collectRabbit(ctx context.Context, result map[instanceKey]entityCounts) {
	instances, err := c.rabbitInstances.GetRabbitInstances(ctx)
	if err != nil {
		log.ErrorC(ctx, "error getting list of rabbit instances, rabbit discrepancy metrics will not be updated: %v", err)
		return
	}

	vhostsInDb, err := c.rabbitVhosts.FindVhostWithSearchForm(ctx, &model.SearchForm{})
	if err != nil {
		log.ErrorC(ctx, "error getting list of vhosts from db, rabbit discrepancy metrics will not be updated: %v", err)
		return
	}
	registeredByInstance := make(map[string]map[string]bool)
	for _, vhost := range vhostsInDb {
		if registeredByInstance[vhost.InstanceId] == nil {
			registeredByInstance[vhost.InstanceId] = make(map[string]bool)
		}
		registeredByInstance[vhost.InstanceId][vhost.Vhost] = true
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeRabbit, instance.GetId()}
		registered := registeredByInstance[instance.GetId()]

		vhostsOnBroker, err := c.rabbitHelperOf(instance).GetAllVhosts(ctx)
		if err != nil {
			log.WarnC(ctx, "error getting list of vhosts from rabbit instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			counts := c.staleCounts(key)
			counts.registered = len(registered)
			result[key] = counts
			continue
		}

		existing := make(map[string]bool, len(vhostsOnBroker))
		for _, vhost := range vhostsOnBroker {
			existing[vhost.Name] = true
		}

		result[key] = compare(registered, existing)
	}
}

// compare calculates discrepancy between entities registered in maas and entities existing on a broker
func compare(registered map[string]bool, existing map[string]bool) entityCounts {
	counts := entityCounts{registered: len(registered), available: true}

	for name := range registered {
		if !existing[name] {
			counts.lost++
		}
	}
	for name := range existing {
		if strings.HasPrefix(name, maasEntityPrefix) && !registered[name] {
			counts.ghost++
		}
	}
	return counts
}

// staleCounts keeps discrepancy numbers of the previous successful calculation, so that
// an unreachable broker doesn't look like every entity has been lost
func (c *MetricCollector) staleCounts(key instanceKey) entityCounts {
	counts := c.lastKnown[key]
	counts.available = false
	return counts
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
