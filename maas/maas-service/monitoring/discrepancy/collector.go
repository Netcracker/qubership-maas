// Package discrepancy exports prometheus metrics describing the difference between
// entities registered in the maas database and entities actually existing on the brokers.
//
// For every registered broker instance three numbers are reported, broken down by the
// maas namespace and tenant the entity belongs to:
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

	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	// entities created by maas are named "maas.<namespace>..."
	maasEntityPrefix = "maas."

	// entities whose namespace can not be determined are reported under this value
	unknownNamespace = "unknown"

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
	GetTopicNames(ctx context.Context, instance *model.KafkaInstance) ([]string, error)
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
	ghost      int
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
		ghostMetric: registerGaugeVec("ghost_entities",
			"number of maas named entities existing on the broker, but not registered in maas database", entityLabels),
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
	c.ghostMetric.Reset()
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
			c.ghostMetric.With(labels).Set(float64(counts.ghost))
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
		registeredByScope := make(map[scopeKey]map[string]bool)
		for _, topic := range topicsInDb {
			addRegistered(registeredByScope, scopeKey{topic.Namespace, topicTenantId(topic)}, topic.Topic)
		}

		topicsOnBroker, err := c.kafkaBroker.GetTopicNames(ctx, &instance)
		if err != nil {
			log.WarnC(ctx, "error getting list of topics from kafka instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredByScope)
			continue
		}

		existing := make(map[string]bool, len(topicsOnBroker))
		for _, name := range topicsOnBroker {
			existing[name] = true
		}

		result[key] = instanceResult{byScope: compareByScope(registeredByScope, existing), reachable: true}
	}
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
	registeredByInstance := make(map[string]map[scopeKey]map[string]bool)
	for _, vhost := range vhostsInDb {
		if registeredByInstance[vhost.InstanceId] == nil {
			registeredByInstance[vhost.InstanceId] = make(map[scopeKey]map[string]bool)
		}
		addRegistered(registeredByInstance[vhost.InstanceId], scopeKey{vhost.Namespace, vhostTenantId(vhost)}, vhost.Vhost)
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeRabbit, instance.GetId()}
		registeredByScope := registeredByInstance[instance.GetId()]

		vhostsOnBroker, err := c.rabbitHelperOf(instance).GetAllVhosts(ctx)
		if err != nil {
			log.WarnC(ctx, "error getting list of vhosts from rabbit instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredByScope)
			continue
		}

		existing := make(map[string]bool, len(vhostsOnBroker))
		for _, vhost := range vhostsOnBroker {
			existing[vhost.Name] = true
		}

		result[key] = instanceResult{byScope: compareByScope(registeredByScope, existing), reachable: true}
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

// addRegistered records an entity name under its scope bucket
func addRegistered(byScope map[scopeKey]map[string]bool, scope scopeKey, name string) {
	if byScope[scope] == nil {
		byScope[scope] = make(map[string]bool)
	}
	byScope[scope][name] = true
}

// compareByScope calculates discrepancy between entities registered in maas and entities
// existing on a broker, broken down by the namespace and tenant of each entity.
func compareByScope(registeredByScope map[scopeKey]map[string]bool, existing map[string]bool) map[scopeKey]nsCounts {
	result := make(map[scopeKey]nsCounts)

	// registered + lost are attributed to the scope stored in the maas database
	registeredNames := make(map[string]bool)
	for scope, names := range registeredByScope {
		counts := nsCounts{registered: len(names)}
		for name := range names {
			registeredNames[name] = true
			if !existing[name] {
				counts.lost++
			}
		}
		result[scope] = counts
	}

	// ghosts exist only on the broker; the namespace is parsed from the maas.<ns>... name,
	// but the tenant id can not be recovered from the name, so it is left empty
	for name := range existing {
		if strings.HasPrefix(name, maasEntityPrefix) && !registeredNames[name] {
			scope := scopeKey{namespace: namespaceFromEntityName(name)}
			counts := result[scope]
			counts.ghost++
			result[scope] = counts
		}
	}
	return result
}

// namespaceFromEntityName extracts the maas namespace from an entity named "maas.<namespace>[.<rest>]"
func namespaceFromEntityName(name string) string {
	rest := strings.TrimPrefix(name, maasEntityPrefix)
	if i := strings.IndexByte(rest, '.'); i >= 0 {
		rest = rest[:i]
	}
	if rest == "" {
		return unknownNamespace
	}
	return rest
}

// staleResult keeps the per-namespace lost/ghost numbers of the previous successful
// calculation (so an unreachable broker doesn't look like every entity has been lost)
// while refreshing the registered counts from the current database view when available.
func (c *MetricCollector) staleResult(key instanceKey, registeredByScope map[scopeKey]map[string]bool) instanceResult {
	byScope := make(map[scopeKey]nsCounts)
	for scope, counts := range c.lastKnown[key] {
		// drop the stale registered count; it is re-derived below when the db view is known
		byScope[scope] = nsCounts{lost: counts.lost, ghost: counts.ghost}
	}
	for scope, names := range registeredByScope {
		counts := byScope[scope]
		counts.registered = len(names)
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
