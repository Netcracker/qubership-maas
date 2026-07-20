// Package discrepancy exports prometheus metrics describing the difference between
// entities registered in the maas database and entities actually existing on the brokers.
//
// For every registered broker instance three numbers are reported, broken down by the
// maas namespace (tenant/service scope) the entity belongs to:
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

	// entities whose namespace can not be determined are reported under this value
	unknownNamespace = "unknown"

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

// nsCounts is a discrepancy snapshot for a single (instance, namespace) pair
type nsCounts struct {
	registered int
	lost       int
	ghost      int
}

// instanceResult holds the per-namespace discrepancy of one broker instance plus its reachability
type instanceResult struct {
	byNamespace map[string]nsCounts
	reachable   bool
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

	// last successfully calculated per-namespace numbers per instance. Kept so that a
	// temporarily unreachable broker doesn't report all its entities as lost.
	lastKnown map[instanceKey]map[string]nsCounts

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
	entityLabels := []string{"broker_type", "broker_id", "entity_namespace"}
	brokerLabels := []string{"broker_type", "broker_id"}
	return &MetricCollector{
		kafkaInstances:  kafkaInstances,
		kafkaTopics:     kafkaTopics,
		kafkaBroker:     kafkaBroker,
		rabbitInstances: rabbitInstances,
		rabbitVhosts:    rabbitVhosts,
		rabbitHelperOf:  rabbitHelperOf,
		collectInterval: collectInterval,
		lastKnown:       make(map[instanceKey]map[string]nsCounts),

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

// Collect recalculates discrepancy for all registered broker instances and publishes
// the result to prometheus. Instances that failed to respond keep their previous numbers
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

	c.lastKnown = make(map[instanceKey]map[string]nsCounts, len(current))
	for key, res := range current {
		c.lastKnown[key] = res.byNamespace
		c.availableMetric.With(prometheus.Labels{
			"broker_type": key.brokerType, "broker_id": key.instanceId,
		}).Set(boolToFloat(res.reachable))

		for ns, counts := range res.byNamespace {
			labels := prometheus.Labels{"broker_type": key.brokerType, "broker_id": key.instanceId, "entity_namespace": ns}
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
		registeredByNs := make(map[string]map[string]bool)
		for _, topic := range topicsInDb {
			addRegistered(registeredByNs, topic.Namespace, topic.Topic)
		}

		topicsOnBroker, err := c.kafkaBroker.GetListTopics(ctx, &instance)
		if err != nil {
			log.WarnC(ctx, "error getting list of topics from kafka instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredByNs)
			continue
		}

		existing := make(map[string]bool, len(topicsOnBroker))
		for name := range topicsOnBroker {
			existing[name] = true
		}

		result[key] = instanceResult{byNamespace: compareByNamespace(registeredByNs, existing), reachable: true}
	}
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
	registeredByInstance := make(map[string]map[string]map[string]bool)
	for _, vhost := range vhostsInDb {
		if registeredByInstance[vhost.InstanceId] == nil {
			registeredByInstance[vhost.InstanceId] = make(map[string]map[string]bool)
		}
		addRegistered(registeredByInstance[vhost.InstanceId], vhost.Namespace, vhost.Vhost)
	}

	for _, instance := range *instances {
		key := instanceKey{brokerTypeRabbit, instance.GetId()}
		registeredByNs := registeredByInstance[instance.GetId()]

		vhostsOnBroker, err := c.rabbitHelperOf(instance).GetAllVhosts(ctx)
		if err != nil {
			log.WarnC(ctx, "error getting list of vhosts from rabbit instance '%v', keeping previous discrepancy numbers: %v", instance.GetId(), err)
			result[key] = c.staleResult(key, registeredByNs)
			continue
		}

		existing := make(map[string]bool, len(vhostsOnBroker))
		for _, vhost := range vhostsOnBroker {
			existing[vhost.Name] = true
		}

		result[key] = instanceResult{byNamespace: compareByNamespace(registeredByNs, existing), reachable: true}
	}
}

// addRegistered records an entity name under its namespace bucket
func addRegistered(byNs map[string]map[string]bool, namespace, name string) {
	if byNs[namespace] == nil {
		byNs[namespace] = make(map[string]bool)
	}
	byNs[namespace][name] = true
}

// compareByNamespace calculates discrepancy between entities registered in maas and
// entities existing on a broker, broken down by the maas namespace of each entity.
func compareByNamespace(registeredByNs map[string]map[string]bool, existing map[string]bool) map[string]nsCounts {
	result := make(map[string]nsCounts)

	// registered + lost are attributed to the namespace stored in the maas database
	registeredNames := make(map[string]bool)
	for ns, names := range registeredByNs {
		counts := nsCounts{registered: len(names)}
		for name := range names {
			registeredNames[name] = true
			if !existing[name] {
				counts.lost++
			}
		}
		result[ns] = counts
	}

	// ghosts exist only on the broker, so their namespace is parsed from the maas.<ns>... name
	for name := range existing {
		if strings.HasPrefix(name, maasEntityPrefix) && !registeredNames[name] {
			ns := namespaceFromEntityName(name)
			counts := result[ns]
			counts.ghost++
			result[ns] = counts
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
func (c *MetricCollector) staleResult(key instanceKey, registeredByNs map[string]map[string]bool) instanceResult {
	byNamespace := make(map[string]nsCounts)
	for ns, counts := range c.lastKnown[key] {
		// drop the stale registered count; it is re-derived below when the db view is known
		byNamespace[ns] = nsCounts{lost: counts.lost, ghost: counts.ghost}
	}
	for ns, names := range registeredByNs {
		counts := byNamespace[ns]
		counts.registered = len(names)
		byNamespace[ns] = counts
	}
	return instanceResult{byNamespace: byNamespace, reachable: false}
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
