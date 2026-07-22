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
	// metadata of topics that exist on the broker, per instance
	metaByInstance map[string]map[string]model.TopicMetadata
	errByInstance  map[string]error
}

func (f *fakeKafkaBroker) GetTopicsMetadata(_ context.Context, instance *model.KafkaInstance, names []string) (map[string]model.TopicMetadata, error) {
	if err, found := f.errByInstance[instance.GetId()]; found {
		return nil, err
	}
	all := f.metaByInstance[instance.GetId()]
	result := make(map[string]model.TopicMetadata)
	for _, name := range names {
		if meta, ok := all[name]; ok {
			result[name] = meta // topics absent from the broker are omitted (treated as lost)
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

// topicReg builds a registered topic with the given expected partitions/replication
func topicReg(name, namespace, tenant, instance string, partitions int32, replication int16) *model.TopicRegistration {
	topic := &model.TopicRegistration{Topic: name, Namespace: namespace, Instance: instance}
	if tenant != "" {
		topic.Classifier = &model.Classifier{Namespace: namespace, TenantId: tenant}
	}
	if partitions > 0 {
		topic.NumPartitions = &partitions
	}
	if replication > 0 {
		topic.ReplicationFactor = &replication
	}
	return topic
}

func meta(partitions int32, replication int16) model.TopicMetadata {
	return model.TopicMetadata{NumPartitions: partitions, ReplicationFactor: replication}
}

func TestTopicSettingsMismatch(t *testing.T) {
	assert.False(t, topicSettingsMismatch(topicReg("t", "ns", "", "i", 3, 2), meta(3, 2)), "in sync")
	assert.True(t, topicSettingsMismatch(topicReg("t", "ns", "", "i", 3, 2), meta(6, 2)), "partition drift")
	assert.True(t, topicSettingsMismatch(topicReg("t", "ns", "", "i", 3, 2), meta(3, 1)), "replication drift")
	// settings maas did not register (nil) are not compared
	assert.False(t, topicSettingsMismatch(topicReg("t", "ns", "", "i", 0, 0), meta(6, 3)), "unregistered settings ignored")
}

func TestCollect(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1", 3, 1),  // ok
			topicReg("maas.core-dev.events", "core-dev", "", "kafka-1", 1, 1),  // lost (not on broker)
			topicReg("maas.core-dev.resized", "core-dev", "", "kafka-1", 3, 1), // mismatched (6 partitions on broker)
			topicReg("maas.payments.tx", "payments", "", "kafka-1", 1, 1),      // ok
		}},
		&fakeKafkaBroker{metaByInstance: map[string]map[string]model.TopicMetadata{
			"kafka-1": {
				"maas.core-dev.orders":  meta(3, 1),
				"maas.core-dev.resized": meta(6, 1),
				"maas.payments.tx":      meta(1, 1),
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

	// kafka core-dev: 3 registered, 1 lost (events), 1 mismatched (resized)
	assert.Equal(t, 3.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.mismatchedMetric, "Kafka", "kafka-1", "core-dev"))
	// kafka payments: 1 registered, in sync
	assert.Equal(t, 1.0, gaugeNs(collector.registeredMetric, "Kafka", "kafka-1", "payments"))
	assert.Equal(t, 0.0, gaugeNs(collector.lostMetric, "Kafka", "kafka-1", "payments"))
	assert.Equal(t, 1.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))

	// rabbit core-dev: 2 registered, 1 lost (gone), mismatched not applicable (always 0)
	assert.Equal(t, 2.0, gaugeNs(collector.registeredMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 1.0, gaugeNs(collector.lostMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 0.0, gaugeNs(collector.mismatchedMetric, "RabbitMQ", "rabbit-1", "core-dev"))
	assert.Equal(t, 1.0, gauge(collector.availableMetric, "RabbitMQ", "rabbit-1"))
}

// an unreachable broker must not be reported as if all its entities disappeared
func TestCollectKeepsPreviousNumbersWhenBrokerIsUnreachable(t *testing.T) {
	ctx := context.Background()
	broker := &fakeKafkaBroker{metaByInstance: map[string]map[string]model.TopicMetadata{
		"kafka-1": {"maas.core-dev.orders": meta(1, 1)},
	}}
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1", 1, 1),
			topicReg("maas.core-dev.events", "core-dev", "", "kafka-1", 1, 1), // lost
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
		&fakeKafkaTopics{topics: []*model.TopicRegistration{topicReg("maas.core-dev.orders", "core-dev", "", "kafka-1", 1, 1)}},
		&fakeKafkaBroker{metaByInstance: map[string]map[string]model.TopicMetadata{"kafka-1": {"maas.core-dev.orders": meta(1, 1)}}},
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
			topicReg("maas.core-dev.t1.orders", "core-dev", "t1", "kafka-1", 1, 1),
		}},
		&fakeKafkaBroker{metaByInstance: map[string]map[string]model.TopicMetadata{
			"kafka-1": {"maas.core-dev.t1.orders": meta(1, 1)},
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

func TestDefaultRabbitHelperFactory(t *testing.T) {
	h := DefaultRabbitHelperFactory(model.RabbitInstance{Id: "x", ApiUrl: "http://localhost:15672/api"})
	assert.NotNil(t, h)
}

func TestStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{topicReg("maas.core-dev.a", "core-dev", "", "kafka-1", 1, 1)}},
		&fakeKafkaBroker{metaByInstance: map[string]map[string]model.TopicMetadata{"kafka-1": {"maas.core-dev.a": meta(1, 1)}}},
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)
	collector.Start(ctx)
	assert.Eventually(t, func() bool {
		return gauge(collector.availableMetric, "Kafka", "kafka-1") == 1.0
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

func gauge(gaugeVec *prometheus.GaugeVec, brokerType string, instanceId string) float64 {
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId))
}

func gaugeNs(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace string) float64 {
	// non-tenant entities have an empty tenant_id label
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, ""))
}

func gaugeScope(gaugeVec *prometheus.GaugeVec, brokerType, instanceId, namespace, tenant string) float64 {
	return testutil.ToFloat64(gaugeVec.WithLabelValues(brokerType, instanceId, namespace, tenant))
}
