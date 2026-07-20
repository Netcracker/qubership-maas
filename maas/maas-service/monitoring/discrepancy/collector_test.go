package discrepancy

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/IBM/sarama"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
)

type fakeKafkaInstances struct {
	instances []model.KafkaInstance
	err       error
}

func (f *fakeKafkaInstances) GetKafkaInstances(_ context.Context) (*[]model.KafkaInstance, error) {
	return &f.instances, f.err
}

type fakeKafkaTopics struct {
	topics []*model.TopicRegistration
	err    error
}

func (f *fakeKafkaTopics) SearchTopicsInDB(_ context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistration, error) {
	// the real KafkaService refuses to run a search without any criteria
	if searchReq.IsEmpty() {
		return nil, errors.New("attempt to search with empty search request")
	}
	if f.err != nil {
		return nil, f.err
	}
	var topics []*model.TopicRegistration
	for _, topic := range f.topics {
		if topic.Instance == searchReq.Instance {
			topics = append(topics, topic)
		}
	}
	return topics, nil
}

type fakeKafkaBroker struct {
	topicsByInstance map[string][]string
	errByInstance    map[string]error
}

func (f *fakeKafkaBroker) GetListTopics(_ context.Context, instance *model.KafkaInstance) (map[string]sarama.TopicDetail, error) {
	if err, found := f.errByInstance[instance.GetId()]; found {
		return nil, err
	}
	result := make(map[string]sarama.TopicDetail)
	for _, topic := range f.topicsByInstance[instance.GetId()] {
		result[topic] = sarama.TopicDetail{}
	}
	return result, nil
}

type fakeRabbitInstances struct {
	instances []model.RabbitInstance
	err       error
}

func (f *fakeRabbitInstances) GetRabbitInstances(_ context.Context) (*[]model.RabbitInstance, error) {
	return &f.instances, f.err
}

type fakeRabbitVhosts struct {
	vhosts []model.VHostRegistration
	err    error
}

func (f *fakeRabbitVhosts) FindVhostWithSearchForm(_ context.Context, _ *model.SearchForm) ([]model.VHostRegistration, error) {
	return f.vhosts, f.err
}

type fakeRabbitHelper struct {
	helper.RabbitHelper
	vhosts []model.VhostInfo
	err    error
}

func (f *fakeRabbitHelper) GetAllVhosts(_ context.Context) ([]model.VhostInfo, error) {
	return f.vhosts, f.err
}

func TestCompareByScope(t *testing.T) {
	tests := []struct {
		name       string
		registered map[scopeKey][]string // scope -> entity names
		existing   []string
		want       map[scopeKey]nsCounts // scope -> expected counts
	}{
		{
			name:       "everything in sync",
			registered: map[scopeKey][]string{{namespace: "core-dev"}: {"maas.core-dev.orders", "maas.core-dev.events"}},
			existing:   []string{"maas.core-dev.orders", "maas.core-dev.events"},
			want:       map[scopeKey]nsCounts{{namespace: "core-dev"}: {registered: 2}},
		},
		{
			name:       "lost attributed to the db scope",
			registered: map[scopeKey][]string{{namespace: "core-dev"}: {"maas.core-dev.orders", "maas.core-dev.events"}},
			existing:   []string{"maas.core-dev.orders"},
			want:       map[scopeKey]nsCounts{{namespace: "core-dev"}: {registered: 2, lost: 1}},
		},
		{
			name:       "tenant scoped entities are counted under their tenant",
			registered: map[scopeKey][]string{{namespace: "core-dev", tenantId: "t1"}: {"maas.core-dev.t1.orders"}},
			existing:   []string{"maas.core-dev.t1.orders"},
			want:       map[scopeKey]nsCounts{{namespace: "core-dev", tenantId: "t1"}: {registered: 1}},
		},
		{
			name:       "ghost attributed to parsed namespace with empty tenant",
			registered: map[scopeKey][]string{{namespace: "core-dev"}: {"maas.core-dev.orders"}},
			existing:   []string{"maas.core-dev.orders", "maas.payments.forgotten"},
			want: map[scopeKey]nsCounts{
				{namespace: "core-dev"}: {registered: 1},
				{namespace: "payments"}: {ghost: 1},
			},
		},
		{
			name:       "foreign entities on broker are not ghosts",
			registered: map[scopeKey][]string{{namespace: "core-dev"}: {"maas.core-dev.orders"}},
			existing:   []string{"maas.core-dev.orders", "__consumer_offsets", "someones-own-topic"},
			want:       map[scopeKey]nsCounts{{namespace: "core-dev"}: {registered: 1}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			byScope := make(map[scopeKey]map[string]bool)
			for scope, names := range test.registered {
				for _, n := range names {
					addRegistered(byScope, scope, n)
				}
			}
			got := compareByScope(byScope, asSet(test.existing))
			assert.Equal(t, test.want, got)
		})
	}
}

func TestNamespaceFromEntityName(t *testing.T) {
	assert.Equal(t, "core-dev", namespaceFromEntityName("maas.core-dev.orders"))
	assert.Equal(t, "core-dev", namespaceFromEntityName("maas.core-dev")) // namespace-only vhost
	assert.Equal(t, unknownNamespace, namespaceFromEntityName("maas."))
}

func TestCollect(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			{Topic: "maas.core-dev.orders", Namespace: "core-dev", Instance: "kafka-1"},
			{Topic: "maas.core-dev.events", Namespace: "core-dev", Instance: "kafka-1"},
			{Topic: "maas.payments.tx", Namespace: "payments", Instance: "kafka-1"},
		}},
		&fakeKafkaBroker{topicsByInstance: map[string][]string{
			// core-dev.events lost; payments.tx present; core-dev.ghost-topic is a ghost
			"kafka-1": {"maas.core-dev.orders", "maas.payments.tx", "maas.core-dev.ghost-topic", "__consumer_offsets"},
		}},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{vhosts: []model.VHostRegistration{
			{Vhost: "maas.core-dev", Namespace: "core-dev", InstanceId: "rabbit-1"},
		}},
		&fakeRabbitHelper{vhosts: []model.VhostInfo{{Name: "maas.core-dev"}, {Name: "/"}}},
	)

	collector.Collect(ctx)

	// kafka core-dev: 2 registered, 1 lost (events), 1 ghost (ghost-topic)
	assert.Equal(t, 2.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.ghostMetric, "Kafka", "kafka-1", "core-dev"))
	// kafka payments: 1 registered, in sync
	assert.Equal(t, 1.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "payments"))
	assert.Equal(t, 0.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "payments"))
	// reachability is per broker, not per namespace
	assert.Equal(t, 1.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))

	// rabbit core-dev: 1 registered, in sync
	assert.Equal(t, 1.0, gaugeNs(collector.registeredMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 0.0, gaugeNs(collector.lostMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 1.0, gauge(collector.availableMetric, "RabbitMQ", "rabbit-1"))
}

// an unreachable broker must not be reported as if all its entities disappeared
func TestCollectKeepsPreviousNumbersWhenBrokerIsUnreachable(t *testing.T) {
	ctx := context.Background()
	broker := &fakeKafkaBroker{topicsByInstance: map[string][]string{
		"kafka-1": {"maas.core-dev.orders"},
	}}
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			{Topic: "maas.core-dev.orders", Namespace: "core-dev", Instance: "kafka-1"},
			{Topic: "maas.core-dev.events", Namespace: "core-dev", Instance: "kafka-1"},
		}},
		broker,
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "core-dev"))

	broker.errByInstance = map[string]error{"kafka-1": errors.New("connection refused")}
	collector.Collect(ctx)

	assert.Equal(t, 2.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "core-dev"), "stale numbers must survive an unreachable broker")
	assert.Equal(t, 0.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))
}

func TestCollectDropsMetricsOfRemovedInstances(t *testing.T) {
	ctx := context.Background()
	instances := &fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}}
	collector := newTestCollector(
		instances,
		&fakeKafkaTopics{topics: []*model.TopicRegistration{{Topic: "maas.core-dev.orders", Namespace: "core-dev", Instance: "kafka-1"}}},
		&fakeKafkaBroker{topicsByInstance: map[string][]string{"kafka-1": {"maas.core-dev.orders"}}},
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)
	assert.Equal(t, 1, testutil.CollectAndCount(collector.registeredMetric))

	instances.instances = nil
	collector.Collect(ctx)
	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
}

func newTestCollector(
	kafkaInstances KafkaInstanceProvider,
	kafkaTopics KafkaTopicProvider,
	kafkaBroker KafkaBrokerLister,
	rabbitInstances RabbitInstanceProvider,
	rabbitVhosts RabbitVhostProvider,
	rabbitHelper helper.RabbitHelper,
) *MetricCollector {
	return NewMetricCollector(
		kafkaInstances, kafkaTopics, kafkaBroker,
		rabbitInstances, rabbitVhosts,
		func(_ model.RabbitInstance) helper.RabbitHelper { return rabbitHelper },
		0,
	)
}

func gauge(gaugeVec *prometheus.GaugeVec, brokerType string, instanceId string) float64 {
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId))
}

func gaugeNs(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace string) float64 {
	// non-tenant entities have an empty tenant_id label
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, ""))
}

func asSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

func gaugeScope(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace, tenant string) float64 {
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, tenant))
}

func TestDefaultRabbitHelperFactory(t *testing.T) {
	h := DefaultRabbitHelperFactory(model.RabbitInstance{Id: "x", ApiUrl: "http://localhost:15672/api"})
	assert.NotNil(t, h)
}

func TestStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{{Topic: "maas.core-dev.a", Namespace: "core-dev", Instance: "kafka-1"}}},
		&fakeKafkaBroker{topicsByInstance: map[string][]string{"kafka-1": {"maas.core-dev.a"}}},
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)
	collector.Start(ctx)
	assert.Eventually(t, func() bool {
		return gauge(collector.availableMetric, "Kafka", "kafka-1") == 1.0
	}, time.Second, 10*time.Millisecond)
}

func TestCollectTenantScoped(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			{Topic: "maas.core-dev.t1.orders", Namespace: "core-dev", Instance: "kafka-1",
				Classifier: &model.Classifier{Namespace: "core-dev", TenantId: "t1"}},
		}},
		&fakeKafkaBroker{topicsByInstance: map[string][]string{"kafka-1": {"maas.core-dev.t1.orders"}}},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{vhosts: []model.VHostRegistration{
			{Vhost: "maas.core-dev.billing", Namespace: "core-dev", InstanceId: "rabbit-1",
				Classifier: `{"name":"billing","namespace":"core-dev","tenantId":"t2"}`},
		}},
		&fakeRabbitHelper{vhosts: []model.VhostInfo{{Name: "maas.core-dev.billing"}}},
	)

	collector.Collect(ctx)

	assert.Equal(t, 1.0, gaugeScope(collector.registeredMetric, "Kafka", "kafka-1", "core-dev", "t1"))
	assert.Equal(t, 1.0, gaugeScope(collector.registeredMetric, "RabbitMQ", "rabbit-1", "core-dev", "t2"))
}

func TestCollectSkipsWhenInstanceListingFails(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{err: errors.New("kafka instances down")},
		&fakeKafkaTopics{},
		&fakeKafkaBroker{},
		&fakeRabbitInstances{err: errors.New("rabbit instances down")},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)

	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
	assert.Equal(t, 0, testutil.CollectAndCount(collector.availableMetric))
}

func TestCollectKafkaDbErrorMarksStale(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{err: errors.New("db down")},
		&fakeKafkaBroker{},
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)

	assert.Equal(t, 0.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))
}

func TestCollectRabbitVhostDbErrorSkips(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{},
		&fakeKafkaTopics{},
		&fakeKafkaBroker{},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{err: errors.New("db down")},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)

	// vhost db read failed before any instance was processed
	assert.Equal(t, 0, testutil.CollectAndCount(collector.availableMetric))
}

func TestCollectRabbitBrokerErrorMarksStale(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{},
		&fakeKafkaTopics{},
		&fakeKafkaBroker{},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{vhosts: []model.VHostRegistration{
			{Vhost: "maas.core-dev.x", Namespace: "core-dev", InstanceId: "rabbit-1"},
		}},
		&fakeRabbitHelper{err: errors.New("broker down")},
	)

	collector.Collect(ctx)

	assert.Equal(t, 1.0, gaugeScope(collector.registeredMetric, "RabbitMQ", "rabbit-1", "core-dev", ""))
	assert.Equal(t, 0.0, gauge(collector.availableMetric, "RabbitMQ", "rabbit-1"))
}
