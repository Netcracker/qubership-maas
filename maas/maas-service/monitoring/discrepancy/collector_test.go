package discrepancy

import (
	"context"
	"errors"
	"testing"

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

func TestCompare(t *testing.T) {
	tests := []struct {
		name       string
		registered []string
		existing   []string
		lost       int
		ghost      int
	}{
		{
			name:       "everything in sync",
			registered: []string{"maas.core-dev.orders", "maas.core-dev.events"},
			existing:   []string{"maas.core-dev.orders", "maas.core-dev.events"},
		},
		{
			name:       "registered in maas but missing on broker",
			registered: []string{"maas.core-dev.orders", "maas.core-dev.events"},
			existing:   []string{"maas.core-dev.orders"},
			lost:       1,
		},
		{
			name:       "maas named entity on broker without registration",
			registered: []string{"maas.core-dev.orders"},
			existing:   []string{"maas.core-dev.orders", "maas.core-dev.forgotten"},
			ghost:      1,
		},
		{
			name:       "foreign entities on broker are not ghosts",
			registered: []string{"maas.core-dev.orders"},
			existing:   []string{"maas.core-dev.orders", "__consumer_offsets", "someones-own-topic"},
		},
		{
			name:       "lost and ghost at the same time",
			registered: []string{"maas.core-dev.orders", "maas.core-dev.events"},
			existing:   []string{"maas.core-dev.orders", "maas.core-dev.forgotten"},
			lost:       1,
			ghost:      1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			counts := compare(asSet(test.registered), asSet(test.existing))

			assert.Equal(t, len(test.registered), counts.registered)
			assert.Equal(t, test.lost, counts.lost)
			assert.Equal(t, test.ghost, counts.ghost)
			assert.True(t, counts.available)
		})
	}
}

func TestCollect(t *testing.T) {
	ctx := context.Background()
	collector := newTestCollector(
		&fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}},
		&fakeKafkaTopics{topics: []*model.TopicRegistration{
			{Topic: "maas.core-dev.orders", Instance: "kafka-1"},
			{Topic: "maas.core-dev.events", Instance: "kafka-1"},
		}},
		&fakeKafkaBroker{topicsByInstance: map[string][]string{
			"kafka-1": {"maas.core-dev.orders", "maas.core-dev.ghost-topic", "__consumer_offsets"},
		}},
		&fakeRabbitInstances{instances: []model.RabbitInstance{{Id: "rabbit-1"}}},
		&fakeRabbitVhosts{vhosts: []model.VHostRegistration{
			{Vhost: "maas.core-dev", InstanceId: "rabbit-1"},
		}},
		&fakeRabbitHelper{vhosts: []model.VhostInfo{{Name: "maas.core-dev"}, {Name: "/"}}},
	)

	collector.Collect(ctx)

	assert.Equal(t, 2.0, gauge(collector.registeredMetric, "Kafka", "kafka-1"))
	assert.Equal(t, 1.0, gauge(collector.lostMetric, "Kafka", "kafka-1"))
	assert.Equal(t, 1.0, gauge(collector.ghostMetric, "Kafka", "kafka-1"))
	assert.Equal(t, 1.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))

	assert.Equal(t, 1.0, gauge(collector.registeredMetric, "RabbitMQ", "rabbit-1"))
	assert.Equal(t, 0.0, gauge(collector.lostMetric, "RabbitMQ", "rabbit-1"))
	assert.Equal(t, 0.0, gauge(collector.ghostMetric, "RabbitMQ", "rabbit-1"))
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
			{Topic: "maas.core-dev.orders", Instance: "kafka-1"},
			{Topic: "maas.core-dev.events", Instance: "kafka-1"},
		}},
		broker,
		&fakeRabbitInstances{},
		&fakeRabbitVhosts{},
		&fakeRabbitHelper{},
	)

	collector.Collect(ctx)
	assert.Equal(t, 1.0, gauge(collector.lostMetric, "Kafka", "kafka-1"))

	broker.errByInstance = map[string]error{"kafka-1": errors.New("connection refused")}
	collector.Collect(ctx)

	assert.Equal(t, 2.0, gauge(collector.registeredMetric, "Kafka", "kafka-1"))
	assert.Equal(t, 1.0, gauge(collector.lostMetric, "Kafka", "kafka-1"), "stale numbers must survive an unreachable broker")
	assert.Equal(t, 0.0, gauge(collector.availableMetric, "Kafka", "kafka-1"))
}

func TestCollectDropsMetricsOfRemovedInstances(t *testing.T) {
	ctx := context.Background()
	instances := &fakeKafkaInstances{instances: []model.KafkaInstance{{Id: "kafka-1"}}}
	collector := newTestCollector(
		instances,
		&fakeKafkaTopics{topics: []*model.TopicRegistration{{Topic: "maas.core-dev.orders", Instance: "kafka-1"}}},
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

func asSet(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}
