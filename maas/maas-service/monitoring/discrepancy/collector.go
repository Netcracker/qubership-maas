// Package discrepancy exports prometheus metrics describing the difference between
// entities registered in the maas database and entities actually existing on the brokers.
//
// For every registered broker instance the following numbers are reported, broken down by the
// maas namespace and tenant the entity belongs to:
//   - registered:  entities maas knows about in its own database
//   - lost:        registered in maas, but missing on the broker
//
// If the data for an instance cannot be read - either from the maas database or from the broker -
// the instance is skipped for the current cycle, the error is logged, and no metrics are emitted
// for it (there is no stale carry-over of previous numbers).
package discrepancy

import (
	"context"
	"errors"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/prometheus/client_golang/prometheus"
)

const (
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

// KafkaBrokerLister returns, for the given topic names, the set that actually exist on the broker.
type KafkaBrokerLister interface {
	GetExistingTopics(ctx context.Context, instance *model.KafkaInstance, topicNames []string) (map[string]bool, error)
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

// scopeKey identifies the namespace and tenant an entity belongs to
type scopeKey struct {
	namespace string
	tenantId  string
}

// nsCounts is a discrepancy snapshot for a single (instance, scope) pair
type nsCounts struct {
	registered int
	lost       int
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

	registeredMetric *prometheus.GaugeVec
	lostMetric       *prometheus.GaugeVec
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
	entityLabels := []string{"broker_type", "broker_id", "entity_namespace", "tenant_id"}
	return &MetricCollector{
		kafkaInstances:  kafkaInstances,
		kafkaTopics:     kafkaTopics,
		kafkaBroker:     kafkaBroker,
		rabbitInstances: rabbitInstances,
		rabbitVhosts:    rabbitVhosts,
		rabbitHelperOf:  rabbitHelperOf,
		collectInterval: collectInterval,

		registeredMetric: registerGaugeVec("registered_entities",
			"number of entities registered in maas database", entityLabels),
		lostMetric: registerGaugeVec("lost_entities",
			"number of entities registered in maas database, but missing on the broker", entityLabels),
	}
}

func registerGaugeVec(name string, help string, labels []string) *prometheus.GaugeVec {
	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "maas",
		Subsystem: "discrepancy",
		Name:      name,
		Help:      help,
	}, labels)

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

// Collect recalculates discrepancy for all registered broker instances.
func (c *MetricCollector) Collect(ctx context.Context) {
	current := make(map[instanceKey]map[scopeKey]nsCounts)

	c.collectKafka(ctx, current)
	c.collectRabbit(ctx, current)

	// instances/namespaces removed from maas (or unreadable this cycle) must not leave stale series behind
	c.registeredMetric.Reset()
	c.lostMetric.Reset()

	for key, byScope := range current {
		for scope, counts := range byScope {
			labels := prometheus.Labels{
				"broker_type": key.brokerType, "broker_id": key.instanceId,
				"entity_namespace": scope.namespace, "tenant_id": scope.tenantId,
			}
			c.registeredMetric.With(labels).Set(float64(counts.registered))
			c.lostMetric.With(labels).Set(float64(counts.lost))
		}
	}
}

func (c *MetricCollector) collectKafka(ctx context.Context, result map[instanceKey]map[scopeKey]nsCounts) {
	instances, err := c.kafkaInstances.GetKafkaInstances(ctx)
	if err != nil {
		log.ErrorC(ctx, "error getting list of kafka instances, kafka discrepancy metrics will not be updated: %v", err)
		return
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeKafka, instance.GetId()}

		topicsInDb, err := c.kafkaTopics.SearchTopicsInDB(ctx, &model.TopicSearchRequest{Instance: instance.GetId()})
		if err != nil {
			log.ErrorC(ctx, "error getting topics of kafka instance '%v' from db, skipping its discrepancy metrics this cycle: %v", instance.GetId(), err)
			continue
		}

		topicNames := make([]string, 0, len(topicsInDb))
		for _, topic := range topicsInDb {
			topicNames = append(topicNames, topic.Topic)
		}

		onBroker, err := c.kafkaBroker.GetExistingTopics(ctx, &instance, topicNames)
		if err != nil {
			log.ErrorC(ctx, "error getting topics from kafka instance '%v', skipping its discrepancy metrics this cycle: %v", instance.GetId(), err)
			continue
		}

		byScope := make(map[scopeKey]nsCounts)
		for _, topic := range topicsInDb {
			scope := scopeKey{topic.Namespace, topicTenantId(topic)}
			counts := byScope[scope]
			counts.registered++
			if !onBroker[topic.Topic] {
				counts.lost++
			}
			byScope[scope] = counts
		}
		result[key] = byScope
	}
}

// topicTenantId returns the tenant id from the topic classifier, empty for non-tenant topics
func topicTenantId(topic *model.TopicRegistration) string {
	if topic.Classifier == nil {
		return ""
	}
	return topic.Classifier.TenantId
}

func (c *MetricCollector) collectRabbit(ctx context.Context, result map[instanceKey]map[scopeKey]nsCounts) {
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
	vhostsByInstance := make(map[string][]model.VHostRegistration)
	for _, vhost := range vhostsInDb {
		vhostsByInstance[vhost.InstanceId] = append(vhostsByInstance[vhost.InstanceId], vhost)
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeRabbit, instance.GetId()}
		registered := vhostsByInstance[instance.GetId()]

		vhostsOnBroker, err := c.rabbitHelperOf(instance).GetAllVhosts(ctx)
		if err != nil {
			log.ErrorC(ctx, "error getting list of vhosts from rabbit instance '%v', skipping its discrepancy metrics this cycle: %v", instance.GetId(), err)
			continue
		}

		existing := make(map[string]bool, len(vhostsOnBroker))
		for _, vhost := range vhostsOnBroker {
			existing[vhost.Name] = true
		}

		// vhosts have no comparable configuration, so only existence (registered/lost) is checked
		byScope := make(map[scopeKey]nsCounts)
		for _, vhost := range registered {
			scope := scopeKey{vhost.Namespace, vhostTenantId(vhost)}
			counts := byScope[scope]
			counts.registered++
			if !existing[vhost.Vhost] {
				counts.lost++
			}
			byScope[scope] = counts
		}
		result[key] = byScope
	}
}

// vhostTenantId returns the tenant id from the vhost classifier, empty for non-tenant vhosts
func vhostTenantId(vhost model.VHostRegistration) string {
	classifier, err := model.ConvertToClassifier(vhost.Classifier)
	if err != nil {
		return ""
	}
	return classifier.TenantId
}
