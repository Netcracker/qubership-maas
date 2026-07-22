// Package discrepancy exports prometheus metrics describing the difference between
// entities registered in the maas database and entities actually existing on the brokers.
//
// For every registered broker instance the following numbers are reported, broken down by the
// maas namespace and tenant the entity belongs to:
//   - registered:  entities maas knows about in its own database
//   - lost:        registered in maas, but missing on the broker
//   - mismatched:  registered in maas and present on the broker, but the broker configuration
//     differs from what maas registered. Kafka only - the number of partitions or the replication
//     factor on the broker does not match the registered topic. RabbitMQ vhosts have no comparable
//     configuration, so the mismatched metric is always zero for them.
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

// KafkaBrokerLister returns the actual metadata (partitions, replication) of the given topics that
// exist on the broker.
type KafkaBrokerLister interface {
	GetTopicsMetadata(ctx context.Context, instance *model.KafkaInstance, topicNames []string) (map[string]model.TopicMetadata, error)
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
	mismatched int
}

// instanceResult holds the per-scope discrepancy of one broker instance plus its reachability
type instanceResult struct {
	byScope   map[scopeKey]nsCounts
	reachable bool
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

	// last successfully calculated per-scope numbers per instance. Kept so that a
	// temporarily unreachable broker doesn't report all its entities as lost.
	lastKnown map[instanceKey]map[scopeKey]nsCounts

	registeredMetric *prometheus.GaugeVec
	lostMetric       *prometheus.GaugeVec
	mismatchedMetric *prometheus.GaugeVec
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
	entityLabels := []string{"broker_type", "broker_id", "entity_namespace", "tenant_id"}
	brokerLabels := []string{"broker_type", "broker_id"}
	return &MetricCollector{
		kafkaInstances:  kafkaInstances,
		kafkaTopics:     kafkaTopics,
		kafkaBroker:     kafkaBroker,
		rabbitInstances: rabbitInstances,
		rabbitVhosts:    rabbitVhosts,
		rabbitHelperOf:  rabbitHelperOf,
		collectInterval: collectInterval,
		lastKnown:       make(map[instanceKey]map[scopeKey]nsCounts),

		registeredMetric: registerGaugeVec("registered_entities",
			"number of entities registered in maas database", entityLabels),
		lostMetric: registerGaugeVec("lost_entities",
			"number of entities registered in maas database, but missing on the broker", entityLabels),
		mismatchedMetric: registerGaugeVec("mismatched_entities",
			"number of entities whose broker configuration differs from what maas registered (kafka only)", entityLabels),
		availableMetric: registerGaugeVec("broker_reachable",
			"1 if the last discrepancy calculation reached the broker, 0 if numbers are stale", brokerLabels),
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
// Instances that failed to respond keep their previous numbers
// and are marked as unreachable.
func (c *MetricCollector) Collect(ctx context.Context) {
	current := make(map[instanceKey]instanceResult)

	c.collectKafka(ctx, current)
	c.collectRabbit(ctx, current)

	// instances/namespaces removed from maas must not leave stale series behind
	c.registeredMetric.Reset()
	c.lostMetric.Reset()
	c.mismatchedMetric.Reset()
	c.availableMetric.Reset()

	c.lastKnown = make(map[instanceKey]map[scopeKey]nsCounts, len(current))
	for key, res := range current {
		c.lastKnown[key] = res.byScope
		c.availableMetric.With(prometheus.Labels{
			"broker_type": key.brokerType, "broker_id": key.instanceId,
		}).Set(boolToFloat(res.reachable))

		for scope, counts := range res.byScope {
			labels := prometheus.Labels{
				"broker_type": key.brokerType, "broker_id": key.instanceId,
				"entity_namespace": scope.namespace, "tenant_id": scope.tenantId,
			}
			c.registeredMetric.With(labels).Set(float64(counts.registered))
			c.lostMetric.With(labels).Set(float64(counts.lost))
			c.mismatchedMetric.With(labels).Set(float64(counts.mismatched))
		}
	}
}

func (c *MetricCollector) collectKafka(ctx context.Context, result map[instanceKey]instanceResult) {
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
			result[key] = c.staleResult(key, nil)
			continue
		}

		topicNames := make([]string, 0, len(topicsInDb))
		for _, topic := range topicsInDb {
			topicNames = append(topicNames, topic.Topic)
		}

		onBroker, err := c.kafkaBroker.GetTopicsMetadata(ctx, &instance, topicNames)
		if err != nil {
			log.WarnC(ctx, "error getting topic metadata from kafka instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredCountsOfTopics(topicsInDb))
			continue
		}

		byScope := make(map[scopeKey]nsCounts)
		for _, topic := range topicsInDb {
			scope := scopeKey{topic.Namespace, topicTenantId(topic)}
			counts := byScope[scope]
			counts.registered++
			switch topic.BrokerStatus(brokerMeta(onBroker, topic.Topic)) {
			case model.StatusAbsent:
				counts.lost++
			case model.StatusMismatched:
				counts.mismatched++
			}
			byScope[scope] = counts
		}
		result[key] = instanceResult{byScope: byScope, reachable: true}
	}
}

// brokerMeta returns topic's broker metadata, or nil if it is not on the broker
func brokerMeta(onBroker map[string]model.TopicMetadata, name string) *model.TopicMetadata {
	if meta, ok := onBroker[name]; ok {
		return &meta
	}
	return nil
}

// registeredCountsOfTopics counts registered topics per scope (used to refresh the registered number
// while keeping the previous lost/mismatched numbers when the broker is unreachable)
func registeredCountsOfTopics(topics []*model.TopicRegistration) map[scopeKey]int {
	counts := make(map[scopeKey]int)
	for _, topic := range topics {
		counts[scopeKey{topic.Namespace, topicTenantId(topic)}]++
	}
	return counts
}

// topicTenantId returns the tenant id from the topic classifier, empty for non-tenant topics
func topicTenantId(topic *model.TopicRegistration) string {
	if topic.Classifier == nil {
		return ""
	}
	return topic.Classifier.TenantId
}

func (c *MetricCollector) collectRabbit(ctx context.Context, result map[instanceKey]instanceResult) {
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
			log.WarnC(ctx, "error getting list of vhosts from rabbit instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredCountsOfVhosts(registered))
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
		result[key] = instanceResult{byScope: byScope, reachable: true}
	}
}

// registeredCountsOfVhosts counts registered vhosts per scope
func registeredCountsOfVhosts(vhosts []model.VHostRegistration) map[scopeKey]int {
	counts := make(map[scopeKey]int)
	for _, vhost := range vhosts {
		counts[scopeKey{vhost.Namespace, vhostTenantId(vhost)}]++
	}
	return counts
}

// vhostTenantId returns the tenant id from the vhost classifier, empty for non-tenant vhosts
func vhostTenantId(vhost model.VHostRegistration) string {
	classifier, err := model.ConvertToClassifier(vhost.Classifier)
	if err != nil {
		return ""
	}
	return classifier.TenantId
}

// staleResult keeps the per-scope lost/mismatched numbers of the previous successful calculation
// (so an unreachable broker doesn't look like every entity has been lost) while refreshing the
// registered counts from the current database view when available.
func (c *MetricCollector) staleResult(key instanceKey, registeredCounts map[scopeKey]int) instanceResult {
	byScope := make(map[scopeKey]nsCounts)
	for scope, counts := range c.lastKnown[key] {
		// drop the stale registered count; it is re-derived below when the db view is known
		byScope[scope] = nsCounts{lost: counts.lost, mismatched: counts.mismatched}
	}
	for scope, n := range registeredCounts {
		counts := byScope[scope]
		counts.registered = n
		byScope[scope] = counts
	}
	return instanceResult{byScope: byScope, reachable: false}
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
