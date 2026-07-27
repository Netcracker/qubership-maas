package discrepancy

import (
	"context"
	"errors"
	"testing"
	"time"

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
	// topics that exist on the broker, per instance
	existsByInstance map[string]map[string]bool
	errByInstance    map[string]error
}

func (f *fakeKafkaBroker) GetExistingTopics(_ context.Context, instance *model.KafkaInstance, names []string) (map[string]bool, error) {
	if err, found := f.errByInstance[instance.GetId()]; found {
		return nil, err
	}
	all := f.existsByInstance[instance.GetId()]
	result := make(map[string]bool)
	for _, name := range names {
		if all[name] {
			result[name] = true // topics absent from the broker are omitted (treated as lost)
		}
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

// topicReg builds a registered topic in the given namespace/tenant on the given instance
func topicReg(name, namespace, tenant, instance string) *model.TopicRegistration {
	topic := &model.TopicRegistration{Topic: name, Namespace: namespace, Instance: instance}
	if tenant != "" {
		topic.Classifier = &model.Classifier{Namespace: namespace, TenantId: tenant}
	}
	return topic
}

func TestCollect(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1"), // ok
			topicReg("maas.core-dev.events", "core-dev", "", "kafka-1"), // lost (not on broker)
			topicReg("maas.payments.tx", "payments", "", "kafka-1"),     // ok
		}},
		&fakeKafkaBroker{existsByInstance: map[string]map[string]bool{
			"kafka-1": {
				"maas.core-dev.orders": true,
				"maas.payments.tx":     true,
			},
		}},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{vhosts: []model.VHostRegistration{
			{Vhost: "maas.core-dev", Namespace: "core-dev", InstanceId: "rabbit-1"},
			{Vhost: "maas.core-dev.gone", Namespace: "core-dev", InstanceId: "rabbit-1"},
		}},
		&fakeRabbitHelper{vhosts: []model.VhostInfo{{Name: "maas.core-dev"}, {Name: "/"}}},
	)

	collector.Collect(ctx)

	// kafka core-dev: 2 registered, 1 lost (events)
	assert.Equal(t, 2.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "core-dev"))
	// kafka payments: 1 registered, in sync
	assert.Equal(t, 1.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "payments"))
	assert.Equal(t, 0.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "payments"))

	// rabbit core-dev: 2 registered, 1 lost (gone)
	assert.Equal(t, 2.0, gaugeNs(collector.registeredMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "RabbitMQ", "rabbit-1", "core-dev"))
}

// an unreachable broker must not leave misleading numbers behind: the instance is skipped entirely
func TestCollectSkipsInstanceWhenBrokerIsUnreachable(t *testing.T) {
	ctx := context.Background()
	broker := &fakeKafkaBroker{existsByInstance: map[string]map[string]bool{
		"kafka-1": {"maas.core-dev.orders": true},
	}}
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1"),
			topicReg("maas.core-dev.events", "core-dev", "", "kafka-1"), // lost
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

	// broker unreadable -> no metrics for the instance this cycle (not stale, not zeroed-in-place)
	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
	assert.Equal(t, 0, testutil.CollectAndCount(collector.lostMetric))
}

func TestCollectDropsMetricsOfRemovedInstances(t *testing.T) {
	ctx := context.Background()
	instances := &fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}}
	collector := newTestCollector(
		instances,
		&fakeKafkaTopics{topics: []*model.TopicRegistration{topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1")}},
		&fakeKafkaBroker{existsByInstance: map[string]map[string]bool{"kafka-1": {"maas.core-dev.orders": true}}},
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

func TestCollectTenantScoped(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			topicReg("maas.core-dev.t1.orders", "core-dev", "t1", "kafka-1"),
		}},
		&fakeKafkaBroker{existsByInstance: map[string]map[string]bool{
			"kafka-1": {"maas.core-dev.t1.orders": true},
		}},
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
}

func TestCollectKafkaDbErrorSkipsInstance(t *testing.T) {
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

	// db unreadable -> no metrics for the instance
	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
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
	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
}

func TestCollectRabbitBrokerErrorSkipsInstance(t *testing.T) {
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

	// broker unreadable -> instance skipped, no metrics emitted
	assert.Equal(t, 0, testutil.CollectAndCount(collector.registeredMetric))
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
		&fakeKafkaTopics{topics: []*model.TopicRegistration{topicReg("maas.core-dev.a", "core-dev", "", "kafka-1")}},
		&fakeKafkaBroker{existsByInstance: map[string]map[string]bool{"kafka-1": {"maas.core-dev.a": true}}},
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)
	collector.Start(ctx)
	assert.Eventually(t, func() bool {
		return gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "core-dev") == 1.0
	}, time.Second, 10*time.Millisecond)
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

func gaugeNs(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace string) float64 {
	// non-tenant entities have an empty tenant_id label
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, ""))
}

func gaugeScope(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace, tenant string) float64 {
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, tenant))
}
