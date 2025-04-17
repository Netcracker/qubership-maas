package helper

import (
	"context"
	"errors"
	"github.com/IBM/sarama"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	mock_sarama "github.com/netcracker/qubership-maas/service/kafka/helper/mock"
	_assert "github.com/stretchr/testify/assert"
	"testing"
	"time"
)

const (
	TestNamespace = "test-namespace"
	TestKafka     = "test_kafka"
	TestKafkaAddr = "test-kafka:9092"
	TestTopicName = "test-namespace.test.topic"
)

var (
	ctx            = context.Background()
	assert         *_assert.Assertions
	mockCtrl       *gomock.Controller
	clusterAdmin   *mock_sarama.MockClusterAdmin
	saramaClient   *mock_sarama.MockSaramaClient
	helper         *HelperImpl
	kafkaInstance  *model.KafkaInstance
	topic          *model.TopicRegistration
	expectedTopic  *model.TopicRegistrationRespDto
	metadata       []*sarama.TopicMetadata
	topicResource  sarama.ConfigResource
	configs        []sarama.ConfigEntry
	TestClassifier = model.Classifier{Name: "test.topic", Namespace: "test-namespace"}
)

func initTest(t *testing.T) {
	assert = _assert.New(t)
	mockCtrl = gomock.NewController(t)
	clusterAdmin = mock_sarama.NewMockClusterAdmin(mockCtrl)
	saramaClient = mock_sarama.NewMockSaramaClient(mockCtrl)
	helper = &HelperImpl{
		KafkaClientTimeout: 0 * time.Nanosecond,
		client:             saramaClient,
	}
	kafkaInstance = &model.KafkaInstance{
		Id:           TestKafka,
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {TestKafkaAddr}},
		MaasProtocol: model.Plaintext,
	}
	topic = &model.TopicRegistration{
		Classifier:  &TestClassifier,
		Topic:       TestTopicName,
		Instance:    TestKafka,
		Namespace:   TestNamespace,
		InstanceRef: kafkaInstance,
	}
	var numPartitions int32 = 2
	var replicationFactor int16 = 2
	flushMs := "1000"
	actualSettings := &model.TopicSettings{
		NumPartitions:     &numPartitions,
		ReplicationFactor: &replicationFactor,
		ReplicaAssignment: map[int32][]int32{0: {0, 1}, 1: {0, 1}},
		Configs:           map[string]*string{"flush.ms": &flushMs},
	}
	expectedTopic = &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {TestKafkaAddr}},
		Name:              TestTopicName,
		Classifier:        TestClassifier,
		Namespace:         TestNamespace,
		Instance:          TestKafka,
		InstanceRef:       kafkaInstance,
		RequestedSettings: topic.GetSettings(),
		ActualSettings:    actualSettings,
	}
	metadata = []*sarama.TopicMetadata{{
		Err:        0,
		Name:       TestTopicName,
		IsInternal: false,
		Partitions: []*sarama.PartitionMetadata{
			{ID: 0, Replicas: []int32{0, 1}},
			{ID: 1, Replicas: []int32{0, 1}},
		}}}
	topicResource = sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: TestTopicName,
	}
	configs = []sarama.ConfigEntry{{Name: "flush.ms", Value: "1000"}}
}

func TestCreateTopicWithDefaults(t *testing.T) {
	initTest(t)
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			CreateTopic(gomock.Eq(TestTopicName), gomock.Eq(buildTopicDetail(topic)), false).
			Return(nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadata, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configs, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	result, err := helper.CreateTopic(ctx, topic)
	assert.Nil(err)
	assert.Equal(*expectedTopic, *result)
}

func TestCreateTopicWithSettings(t *testing.T) {
	initTest(t)
	customFlushMs := "100"
	var numPartitions int32 = 3
	var replicationFactor int16 = 3
	settings := &model.TopicSettings{
		NumPartitions:     &numPartitions,
		ReplicationFactor: &replicationFactor,
		ReplicaAssignment: map[int32][]int32{0: {0, 1, 2}, 1: {2, 0, 1}, 2: {0, 1, 2}},
		Configs:           map[string]*string{"flush.ms": &customFlushMs},
	}
	topic.SetSettings(settings)
	expectedTopic.RequestedSettings = settings
	expectedTopic.ActualSettings = settings
	configs = []sarama.ConfigEntry{{Name: "flush.ms", Value: "100"}}
	metadata = []*sarama.TopicMetadata{{
		Err:        0,
		Name:       TestTopicName,
		IsInternal: false,
		Partitions: []*sarama.PartitionMetadata{
			{ID: 0, Replicas: []int32{0, 1, 2}},
			{ID: 1, Replicas: []int32{2, 0, 1}},
			{ID: 2, Replicas: []int32{0, 1, 2}},
		}}}
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			CreateTopic(gomock.Eq(TestTopicName), gomock.Eq(buildTopicDetail(topic)), false).
			Return(nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadata, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configs, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	result, err := helper.CreateTopic(ctx, topic)
	assert.Nil(err)
	assert.Equal(*expectedTopic, *result)
}

func TestUpdateTopicSettings(t *testing.T) {
	initTest(t)

	configsBefore := []sarama.ConfigEntry{{Name: "flush.ms", Value: "100"}}
	metadataBefore := []*sarama.TopicMetadata{{
		Err:        0,
		Name:       TestTopicName,
		IsInternal: false,
		Partitions: []*sarama.PartitionMetadata{
			{ID: 0, Replicas: []int32{0, 1}},
			{ID: 1, Replicas: []int32{2, 0}},
		}}}

	configsAfter := []sarama.ConfigEntry{{Name: "flush.ms", Value: "1000"}}
	metadataAfter := []*sarama.TopicMetadata{{
		Err:        0,
		Name:       TestTopicName,
		IsInternal: false,
		Partitions: []*sarama.PartitionMetadata{
			{ID: 0, Replicas: []int32{0, 1, 2}},
			{ID: 1, Replicas: []int32{2, 0, 1}},
			{ID: 2, Replicas: []int32{2, 0, 1}},
		}}}

	flushMs := "1000"
	var numPartitions int32 = 3
	var replicationFactor int16 = 3
	settings := &model.TopicSettings{
		NumPartitions:     &numPartitions,
		ReplicationFactor: &replicationFactor,
		ReplicaAssignment: map[int32][]int32{0: {0, 1, 2}, 1: {2, 0, 1}, 2: {2, 0, 1}},
		Configs:           map[string]*string{"flush.ms": &flushMs},
	}
	expectedTopic.RequestedSettings = settings
	expectedTopic.ActualSettings = nil

	partitionAssignment := buildPartitionAssignment(1, settings.ReplicaAssignment)
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadataBefore, nil).
			MinTimes(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configsBefore, nil).
			MinTimes(1),
		clusterAdmin.EXPECT().
			AlterConfig(sarama.TopicResource, gomock.Eq(TestTopicName), expectedTopic.RequestedSettings.Configs, false).
			Return(nil).
			Times(1),
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			CreatePartitions(gomock.Eq(TestTopicName), gomock.Eq(int32(3)), gomock.Eq(partitionAssignment), false).
			Return(nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadataAfter, nil).
			MinTimes(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configsAfter, nil).
			MinTimes(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	err := helper.UpdateTopicSettings(ctx, expectedTopic)
	assert.Nil(err)
	assert.Equal(*expectedTopic.RequestedSettings, *expectedTopic.ActualSettings)
}

func TestUpdateTopicSettingsNoChanges(t *testing.T) {
	initTest(t)
	expectedTopic.RequestedSettings = expectedTopic.ActualSettings
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadata, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configs, nil).
			Times(1),
		clusterAdmin.EXPECT().
			AlterConfig(sarama.TopicResource, gomock.Eq(TestTopicName), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			CreatePartitions(gomock.Eq(TestTopicName), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			AlterPartitionReassignments(gomock.Eq(TestTopicName), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	err := helper.UpdateTopicSettings(ctx, expectedTopic)
	assert.Nil(err)
}

func TestUpdateTopicSettingsShouldRetry(t *testing.T) {
	initTest(t)
	expectedTopic.RequestedSettings = expectedTopic.ActualSettings
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(nil, errors.New("some error")).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return(metadata, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeConfig(topicResource).
			Return(configs, nil).
			Times(1),
		clusterAdmin.EXPECT().
			AlterConfig(sarama.TopicResource, gomock.Eq(TestTopicName), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			CreatePartitions(gomock.Eq(TestTopicName), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			AlterPartitionReassignments(gomock.Eq(TestTopicName), gomock.Any()).
			Return(nil).
			Times(0),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)
	oldTopicUpdateTimeout := helper.KafkaClientTimeout
	helper.KafkaClientTimeout = 10 * time.Second
	defer func() {
		helper.KafkaClientTimeout = oldTopicUpdateTimeout
	}()
	err := helper.UpdateTopicSettings(ctx, expectedTopic)
	assert.Nil(err)

}

func TestDeleteTopic(t *testing.T) {
	initTest(t)
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DeleteTopic(gomock.Eq(TestTopicName)).
			Return(nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{TestTopicName})).
			Return([]*sarama.TopicMetadata{{Err: sarama.ErrUnknownTopicOrPartition}}, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	err := helper.DeleteTopic(ctx, topic)
	assert.Nil(err)
}

func TestDeleteTopic_TopicNotFound(t *testing.T) {
	initTest(t)
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DeleteTopic(gomock.Eq(TestTopicName)).
			Return(sarama.ErrUnknownTopicOrPartition).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	err := helper.DeleteTopic(ctx, topic)
	assert.Nil(err)
}

func TestPropagateConfigsToTopicDetail(t *testing.T) {
	assertion := _assert.New(t)

	var numPartitions int32 = 2
	var replicationFactor int16 = 2
	var replicaAssignment = map[int32][]int32{0: {0, 1, 2}, 1: {0, 1, 2}}
	flushMs := "10000"
	configs := map[string]*string{
		"flush.ms":                            &flushMs,
		"confluent.key.subject.name.strategy": nil,
	}
	topic := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "myName", Namespace: "local"},
		Topic:      "local.__default.myName",
		Instance:   "localKafka",
		Namespace:  "local",
		TopicSettings: model.TopicSettings{
			NumPartitions:     &numPartitions,
			ReplicationFactor: &replicationFactor,
			ReplicaAssignment: replicaAssignment,
			Configs:           configs,
		},
	}

	topicDetail := buildTopicDetail(topic)

	assertion.Equal(*topic.NumPartitions, topicDetail.NumPartitions)
	assertion.Equal(*topic.ReplicationFactor, topicDetail.ReplicationFactor)
	assertion.Equal(topic.ReplicaAssignment, topicDetail.ReplicaAssignment)
	assertion.Equal(topic.Configs, topicDetail.ConfigEntries)
}

func TestDefaultConfigsForTopicDetail(t *testing.T) {
	assertion := _assert.New(t)

	topic := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "myName", Namespace: "local"},
		Topic:      "local.__default.myName",
		Instance:   "localKafka",
		Namespace:  "local",
		TopicSettings: model.TopicSettings{
			NumPartitions:     nil,
			MinNumPartitions:  nil,
			ReplicationFactor: nil,
			ReplicaAssignment: nil,
			Configs:           nil,
		},
	}

	topicDetail := buildTopicDetail(topic)

	assertion.Equal(int32(-1), topicDetail.NumPartitions)
	assertion.Equal(int16(-1), topicDetail.ReplicationFactor)
	assertion.Empty(topicDetail.ReplicaAssignment)
	assertion.Empty(topicDetail.ConfigEntries)
}

func TestBuildPartitionAssignment(t *testing.T) {
	assertion := _assert.New(t)

	topic := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "myName", Namespace: "local"},
		Topic:      "local.__default.myName",
		Instance:   "localKafka",
		Namespace:  "local",
		TopicSettings: model.TopicSettings{
			NumPartitions:     nil,
			MinNumPartitions:  nil,
			ReplicationFactor: nil,
			ReplicaAssignment: nil,
			Configs:           nil,
		},
	}

	assignment := buildPartitionAssignment(0, topic.ReplicaAssignment)
	assertion.Empty(assignment)
	assignment = buildPartitionAssignment(5, topic.ReplicaAssignment)
	assertion.Empty(assignment)

	topic.ReplicaAssignment = map[int32][]int32{0: {1, 2}, 1: {1, 2}, 2: {2, 1}}
	assignment = buildPartitionAssignment(0, topic.ReplicaAssignment)
	assertion.Empty(assignment)
	assignment = buildPartitionAssignment(3, topic.ReplicaAssignment)
	assertion.Equal([][]int32{0: {1, 2}, 1: {1, 2}, 2: {2, 1}}, assignment)
	assignment = buildPartitionAssignment(1, topic.ReplicaAssignment)
	assertion.Equal([][]int32{0: {2, 1}}, assignment)
}

func TestIsTopicExistingOnKafka_True(t *testing.T) {
	initTest(t)
	var returnedTopicMetadata = &sarama.TopicMetadata{
		Err:        0,
		Name:       topic.Topic,
		IsInternal: false,
		Partitions: nil,
	}
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{topic.Topic})).
			Return([]*sarama.TopicMetadata{returnedTopicMetadata}, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	result, err := helper.DoesTopicExistOnKafka(ctx, kafkaInstance, topic.Topic)
	assert.Nil(err)
	assert.True(result)
}

func TestIsTopicExistingOnKafka_False(t *testing.T) {
	initTest(t)
	var returnedTopicMetadata = &sarama.TopicMetadata{
		Err:        3,
		Name:       topic.Topic,
		IsInternal: false,
		Partitions: nil,
	}
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{topic.Topic})).
			Return([]*sarama.TopicMetadata{returnedTopicMetadata}, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	result, err := helper.DoesTopicExistOnKafka(ctx, kafkaInstance, topic.Topic)
	assert.Nil(err)
	assert.False(result)
}

func TestIsTopicExistingOnKafka_Error(t *testing.T) {
	initTest(t)
	var returnedTopicMetadata = &sarama.TopicMetadata{
		Err:        1,
		Name:       topic.Topic,
		IsInternal: false,
		Partitions: nil,
	}
	gomock.InOrder(
		saramaClient.EXPECT().
			NewClusterAdmin(gomock.Eq([]string{TestKafkaAddr}), &VersionMatcher{Expected: sarama.V2_8_0_0.String()}).
			Return(clusterAdmin, nil).
			Times(1),
		clusterAdmin.EXPECT().
			DescribeTopics(gomock.Eq([]string{topic.Topic})).
			Return([]*sarama.TopicMetadata{returnedTopicMetadata}, nil).
			Times(1),
		clusterAdmin.EXPECT().
			Close().
			Return(nil).
			Times(1),
	)

	result, err := helper.DoesTopicExistOnKafka(ctx, kafkaInstance, topic.Topic)
	assert.NotNil(err)
	assert.False(result)
}

type VersionMatcher struct {
	Expected string
}

func (matcher *VersionMatcher) Matches(x interface{}) bool {
	return x.(*sarama.Config).Version.String() == matcher.Expected
}

// String describes what the matcher matches.
func (matcher *VersionMatcher) String() string {
	return "Kafka versions in sarama config must match the expected version value"
}
