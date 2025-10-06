package helper

import (
	"context"
	"errors"
	"fmt"
	"github.com/IBM/sarama"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/prometheus/client_golang/prometheus"
	"strings"
	"time"
)

var log = logging.GetLogger("kafka-helper")

var ErrTopicDeletionDisabled = errors.New("kafka: topic deletion is disabled ('delete.topic.enable' is unset in kafka server properties)")
var ErrInvalidDeleteRequest = errors.New("kafka: received INVALID_REQUEST response from kafka, make sure that 'delete.topic.enable' is set to true")
var ErrSettingsAreNotUpdated = errors.New("kafka: topic settings are not updated (possibly the new values resulted in the same settings as were before changes)")
var ErrInvalidPartitionsUpdate = errors.New("kafka: number of partitions for topic is bigger than actual")

var createMethodMetric = newMetricCounter("createTopic")
var deleteMethodMetric = newMetricCounter("deleteTopic")
var updateMethodMetric = newMetricCounter("updateTopic")
var isTopicExistsMethodMetric = newMetricCounter("isTopicExists")
var settingsMethodMetric = newMetricCounter("topicSettings")
var listTopicsMethodMetric = newMetricCounter("listTopics")
var bulkTopicSettingsMetric = newMetricCounter("bulkTopicSetting")
var healthMetric = newMetricCounter("health")

func CreateKafkaHelper(ctx context.Context) Helper {
	sarama.Logger = NewSaramaLogger(ctx, logging.GetLogger("sarama"))
	topicClientTimeoutStr := configloader.GetKoanf().String("kafka.topics.update.timeout")
	if topicClientTimeoutStr == "" {
		topicClientTimeoutStr = configloader.GetKoanf().String("kafka.client.timeout")
	} else {
		log.WarnC(ctx, "You're using deprecated parameter 'kafka.topics.update.timeout', use 'kafka.client.timeout' instead.")
	}

	clientTimeout, err := time.ParseDuration(topicClientTimeoutStr)
	if err != nil {
		log.PanicC(ctx, "wrong parameter value for 'kafka.client.timeout': '%v'. It should have time format, for example '10s'", topicClientTimeoutStr)
	}

	return NewHelperImpl(clientTimeout)
}

//go:generate mockgen -source=helper.go -destination=mock/helper.go
type Helper interface {
	CreateTopic(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error)
	DeleteTopic(ctx context.Context, topic *model.TopicRegistration) error
	UpdateTopicSettings(ctx context.Context, topic *model.TopicRegistrationRespDto) error
	GetTopicSettings(ctx context.Context, topic *model.TopicRegistrationRespDto) error
	GetListTopics(ctx context.Context, instance *model.KafkaInstance) (map[string]sarama.TopicDetail, error)
	DoesTopicExistOnKafka(ctx context.Context, instance *model.KafkaInstance, topicName string) (bool, error)
	BulkGetTopicSettings(ctx context.Context, topics []*model.TopicRegistrationRespDto) error

	CheckHealth(ctx context.Context, instance *model.KafkaInstance) error
}

type HelperImpl struct {
	KafkaClientTimeout time.Duration
	client             SaramaClient
}

func NewHelperImpl(kafkaClientTimeout time.Duration) *HelperImpl {
	return &HelperImpl{KafkaClientTimeout: kafkaClientTimeout, client: &SaramaClientImpl{}}
}

func (helper HelperImpl) CheckHealth(ctx context.Context, kafkaInstance *model.KafkaInstance) (ret error) {
	defer func() {
		if r := recover(); r != nil {
			log.ErrorC(healthCtx, "Recovered from health panic: %v", r)
			ret = fmt.Errorf("%v", r)
		}
	}()

	admin, err := helper.createClusterAdmin(ctx, kafkaInstance)
	if err != nil {
		log.WarnC(ctx, "Failed to create kafka admin client for %+v: %v", kafkaInstance, err)
		return err
	}
	defer admin.Close()

	_, err = admin.DescribeTopics([]string{dummyNonExistenceTopic})
	if err != nil {
		log.WarnC(ctx, "Kafka instance %+v health check returned error: %v", kafkaInstance, err)
		return err
	}
	log.DebugC(ctx, "Kafka instance %+v is healthy", kafkaInstance)
	return nil
}

func (helper *HelperImpl) CreateTopic(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
	return measureTimeValue(createMethodMetric, func() (*model.TopicRegistrationRespDto, error) {
		if topic.ExternallyManaged {
			return topic.ToResponseDto(), nil
		}
		admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
		if err != nil {
			return nil, err
		}
		defer admin.Close()

		topicDetail := buildTopicDetail(topic)
		log.InfoC(ctx, "Trying to create topic: `%v', details: %+v", topic.Topic, topicDetail)
		if err := admin.CreateTopic(topic.Topic, topicDetail, false); err == nil {
			log.InfoC(ctx, "Topic `%v' has been created", topic.Topic)
		} else {
			log.ErrorC(ctx, "Failed to create kafka topic %s in instance %s: %v", topic.Topic, topic.Instance, err)
			return nil, err
		}

		result := topic.ToResponseDto()
		if err := helper.enrichNewTopicSettings(ctx, admin, result); err != nil {
			log.ErrorC(ctx, "Deleting newly created topic %s due to error: %v", topic.Topic, err)
			_ = helper.DeleteTopic(ctx, topic)
			return nil, err
		}

		log.InfoC(ctx, "Kafka topic %s has been successfully created in instance %s", topic.Topic, topic.Instance)
		return result, nil
	})
}

func (helper *HelperImpl) DeleteTopic(ctx context.Context, topic *model.TopicRegistration) error {
	return measureTime(deleteMethodMetric, func() error {
		if topic.ExternallyManaged {
			log.InfoC(ctx, "Topic `%v' is externally managed, skip deletion", topic.Topic)
			return nil
		}
		admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
		if err != nil {
			return err
		}
		defer admin.Close()
		log.InfoC(ctx, "Try delete topic: `%v'", topic.Topic)

		if err := admin.DeleteTopic(topic.Topic); err != nil {
			log.ErrorC(ctx, "Failed to delete kafka topic %s in instance %s: %v", topic.Topic, topic.Instance, err)
			switch { // proxying sarama errors so other MaaS packages do not depend on sarama kafka client
			case errors.Is(err, sarama.ErrTopicDeletionDisabled):
				return ErrTopicDeletionDisabled
			case errors.Is(err, sarama.ErrInvalidRequest):
				return ErrInvalidDeleteRequest
			case errors.Is(err, sarama.ErrUnknownTopicOrPartition):
				log.Warnf("Topic %s is not found in broker %v, bu topic was deleted from database. Continue processing delete request.", topic.Topic, topic.InstanceRef.Addresses)
				return nil
			default:
				return fmt.Errorf("unexepcted error during topic deletion: %w", err)
			}
		}

		return utils.Retry(ctx, 30*time.Second, 1*time.Second, func(ctx context.Context) error {
			result, err := admin.DescribeTopics([]string{topic.Topic})
			if err != nil {
				return err
			}
			for _, desc := range result {
				if errors.Is(desc.Err, sarama.ErrUnknownTopicOrPartition) {
					log.InfoC(ctx, "Kafka topic %s has been successfully deleted in instance %s", topic.Topic, topic.Instance)
					return nil
				}
			}
			return fmt.Errorf("topic deletion timed out: %v", topic.Topic)
		})
	})
}

func (helper *HelperImpl) UpdateTopicSettings(ctx context.Context, topic *model.TopicRegistrationRespDto) error {
	return measureTime(updateMethodMetric, func() error {
		log.InfoC(ctx, "UpdateTopicSettings started")
		if topic.ExternallyManaged {
			log.InfoC(ctx, "topic is externally managed, UpdateTopicSettings skipped")
			return nil
		}
		admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
		if err != nil {
			return err
		}
		defer admin.Close()

		requestedSettings := topic.RequestedSettings
		log.DebugC(ctx, "requestedSettings %+v", requestedSettings)

		// load actual settings from kafka
		originalSettings, err := utils.RetryValue(ctx, helper.KafkaClientTimeout, 200*time.Millisecond, func(ctx context.Context) (*model.TopicSettings, error) {
			return helper.getTopicSettings(ctx, admin, topic.Name)
		})
		if err != nil {
			return utils.LogError(log, ctx, "error getting topic settings: %w", err)
		}
		log.DebugC(ctx, "originalSettings %+v", originalSettings)

		// update configs if needed
		topicUpdated, err := helper.updateTopicConfigs(ctx, admin, topic.Name, originalSettings, requestedSettings)
		if err != nil {
			return utils.LogError(log, ctx, "error update topic settings: %w", err)
		}

		// update partitions if needed
		if (!model.IsEmpty(requestedSettings.NumPartitions) && *originalSettings.NumPartitions != *requestedSettings.NumPartitions) ||
			!model.IsEmpty(requestedSettings.MinNumPartitions) && !equalsInt32(requestedSettings.MinNumPartitions, originalSettings.MinNumPartitions) {
			partitionsCreated, err := helper.createPartitionsForTopic(ctx, topic, originalSettings, requestedSettings)
			if err != nil {
				return err
			}
			topicUpdated = topicUpdated || partitionsCreated
		} else {
			if requestedSettings.ReplicaAssignment != nil {
				assignmentsUpdated, err := helper.alterPartitionReassignmentsForTopic(ctx, topic, originalSettings, requestedSettings)
				if err != nil {
					return err
				}
				topicUpdated = topicUpdated || assignmentsUpdated
			} else if !model.IsEmpty(requestedSettings.ReplicationFactor) {
				err = helper.alterPartitionReassignmentsByReplicationFactor(ctx, topic)
				if err != nil {
					return err
				}
				topicUpdated = true
			}
		}

		if topicUpdated {
			// load actual (effective) settings from kafka after update operation is performed
			if err := helper.enrichTopicSettingsWhenApplied(ctx, admin, originalSettings, topic); err != nil {
				log.ErrorC(ctx, "Failed to load topic %s metadata from kafka after settings have been applied: %v", topic.Name, err)
				return err
			}
		}
		log.InfoC(ctx, "Kafka topic %s has been successfully updated in instance %s", topic.Name, topic.Instance)
		return nil
	})
}

func (helper *HelperImpl) updateTopicConfigs(ctx context.Context, admin sarama.ClusterAdmin, topicName string, originalSettings, requestedSettings *model.TopicSettings) (bool, error) {
	if requestedSettings.Configs == nil || utils.CheckForChangesByRequest(originalSettings.Configs, requestedSettings.Configs) {
		return false, nil
	}
	log.DebugC(ctx, "Updating configs for topic %v", topicName)
	if err := admin.AlterConfig(sarama.TopicResource, topicName, requestedSettings.Configs, false); err != nil {
		log.ErrorC(ctx, "Failed to alter topic %s configs: %v", topicName, err)
		return false, err
	}
	log.InfoC(ctx, "Topic %s configs have been updated with new configs: %+v", topicName, requestedSettings.Configs)
	return true, nil
}

func (helper *HelperImpl) createPartitionsForTopic(ctx context.Context, topic *model.TopicRegistrationRespDto, originalSettings, requestedSettings *model.TopicSettings) (bool, error) {
	log.InfoC(ctx, "createPartitionsForTopic started")
	if (*requestedSettings.NumPartitions != 0 && *requestedSettings.NumPartitions < *originalSettings.NumPartitions) &&
		(requestedSettings.MinNumPartitions == nil || *requestedSettings.MinNumPartitions == 0) {
		errMsg := fmt.Sprintf("Invalid update settings request for topic %v: %v. Current num partitions is '%v', requested num partitions is '%v'", topic.Name, ErrInvalidPartitionsUpdate, *originalSettings.NumPartitions, *requestedSettings.NumPartitions)
		log.ErrorC(ctx, errMsg)
		return false, fmt.Errorf("%v: %w", errMsg, ErrInvalidPartitionsUpdate)
	}

	numPartitions := requestedSettings.ActualNumPartitions()
	newPartitionsNum := numPartitions - originalSettings.ActualNumPartitions()
	if newPartitionsNum <= 0 {
		log.InfoC(ctx, "No need to increase partitions count: was=%d, requested=%s", numPartitions, newPartitionsNum)
		return false, nil
	}
	log.DebugC(ctx, "numPartitions %v", numPartitions)
	log.DebugC(ctx, "newPartitionsNum %v", newPartitionsNum)
	partitionAssignment := buildPartitionAssignment(newPartitionsNum, requestedSettings.ReplicaAssignment)
	log.DebugC(ctx, "partitionAssignment %v", partitionAssignment)

	// need higher API version so open another ClusterAdmin
	admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
	if err != nil {
		return false, err
	}
	defer admin.Close()

	if err := admin.CreatePartitions(topic.Name, numPartitions, partitionAssignment, false); err != nil {
		log.ErrorC(ctx, "Failed to increase the number of partitions of the topics %s: %v", topic.Name, err)
		return false, err
	}
	log.InfoC(ctx, "Created %v new partitions for topic %s", newPartitionsNum, topic.Name)
	return true, nil
}

func (helper *HelperImpl) alterPartitionReassignmentsForTopic(ctx context.Context, topic *model.TopicRegistrationRespDto, originalSettings, requestedSettings *model.TopicSettings) (bool, error) {
	log.InfoC(ctx, "alterPartitionReassignmentsForTopic started")
	replicaAssignmentEquals := true
	if len(requestedSettings.ReplicaAssignment) != len(originalSettings.ReplicaAssignment) {
		replicaAssignmentEquals = false
	} else if len(requestedSettings.ReplicaAssignment) > 0 {
		for key, val := range requestedSettings.ReplicaAssignment {
			if anotherVal, found := originalSettings.ReplicaAssignment[key]; !found || !utils.SlicesAreEqualInt32(val, anotherVal) {
				replicaAssignmentEquals = false
			}
		}
	}
	if replicaAssignmentEquals {
		return false, nil
	}
	// need higher API version so open another ClusterAdmin
	admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
	if err != nil {
		return false, err
	}
	defer admin.Close()

	log.DebugC(ctx, "topic registration: %+v", topic)
	log.DebugC(ctx, "topic name: %+v", topic.Name)
	assignment := buildPartitionAssignment(originalSettings.ActualNumPartitions(), requestedSettings.ReplicaAssignment)
	log.DebugC(ctx, "assignment: %+v", assignment)

	if err := admin.AlterPartitionReassignments(topic.Name, assignment); err != nil {
		log.ErrorC(ctx, "Failed to alter partition reassignments for topic %s: %v", topic.Name, err)
		return false, err
	}
	log.InfoC(ctx, "Updated partition reassignment for topic %s with values %+v", topic.Name, assignment)
	return true, nil
}

func (helper *HelperImpl) alterPartitionReassignmentsByReplicationFactor(ctx context.Context, topic *model.TopicRegistrationRespDto) error {
	admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
	if err != nil {
		return err
	}
	defer admin.Close()

	brokers, _, err := admin.DescribeCluster()
	if err != nil {
		return err
	}
	brokerIds := make([]int32, len(brokers))
	for i, broker := range brokers {
		brokerIds[i] = broker.ID()
	}

	assignment := make([][]int32, topic.RequestedSettings.ActualNumPartitions())
	for i := 0; i < int(topic.RequestedSettings.ActualNumPartitions()); i++ {
		assignment[i], err = utils.RandomSubSequence(brokerIds, int(*topic.RequestedSettings.ReplicationFactor))
		if err != nil {
			return fmt.Errorf("can not select brokers for topic '%s'; requested replication factor %d; avaliable brokers count %d: %w",
				topic.Name, *topic.RequestedSettings.ReplicationFactor, len(brokers), err)
		}
		log.InfoC(ctx, "selected brokers for '%d' partition: %v", i, assignment[i])
	}

	if err = admin.AlterPartitionReassignments(topic.Name, assignment); err != nil {
		log.ErrorC(ctx, "Failed to alter partition reassignments for topic %s: %v", topic.Name, err)
		return err
	}
	return nil
}

// GetTopicSettings loads actual (effective) topic settings from Kafka.
// TODO get methods should return value
func (helper *HelperImpl) GetTopicSettings(ctx context.Context, topic *model.TopicRegistrationRespDto) error {
	return measureTime(settingsMethodMetric, func() error {
		if topic.ExternallyManaged {
			return nil
		}
		log.DebugC(ctx, "Getting actual topics %s settings from kafka", topic.Name)
		admin, err := helper.createClusterAdmin(ctx, topic.InstanceRef)
		if err != nil {
			return err
		}
		defer admin.Close()

		return helper.enrichNewTopicSettings(ctx, admin, topic)
	})
}

// enrichNewTopicSettings gets settings for newly created topic from kafka. Retries until topic is created and acknowledged by all brokers.
func (helper *HelperImpl) enrichNewTopicSettings(ctx context.Context, admin sarama.ClusterAdmin, topic *model.TopicRegistrationRespDto) error {
	// retry up to 10 seconds (some brokers might still not have acknowledged that new topic was created)
	var err error
	topic.ActualSettings, err = utils.RetryValue(ctx, helper.KafkaClientTimeout, 200*time.Millisecond, func(ctx context.Context) (*model.TopicSettings, error) {
		return helper.getTopicSettings(ctx, admin, topic.Name)
	})
	if err != nil {
		log.ErrorC(ctx, "Failed to obtain kafka topic %s metadata from instance %s: %v", topic.Name, topic.Instance, err)
		return err
	}
	return nil
}

// enrichExistingTopicSettings gets settings for existing topic from kafka without retry.
func (helper *HelperImpl) enrichExistingTopicSettings(ctx context.Context, admin sarama.ClusterAdmin, topic *model.TopicRegistrationRespDto) error {
	settings, err := helper.getTopicSettings(ctx, admin, topic.Name)
	if err != nil {
		log.ErrorC(ctx, "Failed to obtain kafka topic %s metadata from instance %s: %v", topic.Name, topic.Instance, err)
		return err
	}
	topic.ActualSettings = settings
	return nil
}

// enrichTopicSettingsWhenApplied gets settings for topic from kafka and compares result to the previous version of settings.
// Retries until new settings are applied, and then enriches passed topic structure with new settings.
func (helper *HelperImpl) enrichTopicSettingsWhenApplied(ctx context.Context, admin sarama.ClusterAdmin, oldSettings *model.TopicSettings, topic *model.TopicRegistrationRespDto) error {
	// retry up to 10 seconds (some brokers might still not have acknowledged that topic settings have been changed)
	settings, err := utils.RetryValue(ctx, helper.KafkaClientTimeout, 200*time.Millisecond, func(ctx context.Context) (*model.TopicSettings, error) {
		receivedSettings, err := helper.getTopicSettings(ctx, admin, topic.Name)
		if err != nil {
			return nil, err
		}
		if receivedSettings.Equals(oldSettings) {
			return receivedSettings, ErrSettingsAreNotUpdated
		}
		return receivedSettings, nil
	})
	if err == nil {
		topic.ActualSettings = settings
	} else {
		log.WarnC(ctx, "Failed to obtain kafka topic %s metadata from instance %s after settings update: %v", topic.Name, topic.Instance, err)
		if settings != nil {
			topic.ActualSettings = settings
		}
	}
	return nil
}

func (helper *HelperImpl) getTopicSettings(ctx context.Context, admin sarama.ClusterAdmin, topic string) (*model.TopicSettings, error) {
	metadata, err := admin.DescribeTopics([]string{topic})
	if err != nil {
		log.ErrorC(ctx, "Failed to obtain kafka topic %s metadata from kafka: %v", topic, err)
		return nil, err
	} else if len(metadata) != 1 || metadata[0] == nil {
		err = errors.New("kafka: metadata for created topic not found")
		log.ErrorC(ctx, "Failed to obtain kafka topic %s metadata from kafka: %v", topic, err)
		return nil, err
	} else if metadata[0].Err != 0 {
		err = errors.New(fmt.Sprintf("kafka: failed to get created topic description (kafka err code %v)", metadata[0].Err))
		log.ErrorC(ctx, "Error in kafka topic %s metadata: %v", topic, err)
		return nil, err
	}
	topicMetadata := metadata[0]

	numPartitions := int32(len(topicMetadata.Partitions))
	result := model.TopicSettings{
		NumPartitions: &numPartitions,
	}
	if len(topicMetadata.Partitions) > 0 {
		result.ReplicaAssignment = map[int32][]int32{}
		for _, partition := range topicMetadata.Partitions {
			result.ReplicaAssignment[partition.ID] = partition.Replicas
		}
		replicationFactor := int16(len(topicMetadata.Partitions[0].Replicas))
		result.ReplicationFactor = &replicationFactor
	}

	topicResource := sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: topic,
	}
	configs, err := admin.DescribeConfig(topicResource)
	if err != nil {
		log.ErrorC(ctx, "Failed to obtain kafka topic %s configs from kafka. Error: %v", topic, err)
		return nil, err
	}
	result.Configs = make(map[string]*string, len(configs))
	for _, configEntry := range configs {
		configValue := configEntry.Value
		result.Configs[configEntry.Name] = &configValue
	}
	return &result, nil
}

func (helper *HelperImpl) GetListTopics(ctx context.Context, instance *model.KafkaInstance) (map[string]sarama.TopicDetail, error) {
	return measureTimeValue(listTopicsMethodMetric, func() (map[string]sarama.TopicDetail, error) {
		admin, err := helper.createClusterAdmin(ctx, instance)
		if err != nil {
			log.ErrorC(ctx, "Failed to connect to kafka instance %s: %v", instance, err)
			return nil, err
		}
		defer admin.Close()

		if topics, err := admin.ListTopics(); err == nil {
			return topics, nil
		} else {
			log.ErrorC(ctx, "Failed to get list of topics from kafka instance %s: %v", instance, err)
			return nil, err
		}
	})
}

func (helper *HelperImpl) DoesTopicExistOnKafka(ctx context.Context, instance *model.KafkaInstance, topicName string) (bool, error) {
	return measureTimeValue(isTopicExistsMethodMetric, func() (bool, error) {
		admin, err := helper.createClusterAdmin(ctx, instance)
		if err != nil {
			return false, utils.LogError(log, ctx, "Failed to connect to kafka instance %s: %v", instance, err)
		}
		defer admin.Close()
		if topics, err := admin.DescribeTopics([]string{topicName}); err == nil {
			switch topics[0].Err { //https://cwiki.apache.org/confluence/display/KAFKA/A+Guide+To+The+Kafka+Protocol#AGuideToTheKafkaProtocol-ErrorCodes
			case 0:
				return true, nil
			case 3:
				return false, nil
			default:
				return false, utils.LogError(log, ctx, "received description of topic %s from kafka instance %s with error: %s", topicName, instance, topics[0].Err.Error())
			}
		} else {
			return false, utils.LogError(log, ctx, "Failed to receive description of topic %s from kafka instance %s: %v", topicName, instance, err)
		}
	})
}

func (helper *HelperImpl) BulkGetTopicSettings(ctx context.Context, topics []*model.TopicRegistrationRespDto) error {
	return measureTime(bulkTopicSettingsMetric, func() error {
		log.DebugC(ctx, "Bulk getting actual topics settings from kafka")

		topicsByInstances := make(map[string][]*model.TopicRegistrationRespDto)
		for _, topic := range topics {
			if _, found := topicsByInstances[topic.Instance]; !found {
				topicsByInstances[topic.Instance] = make([]*model.TopicRegistrationRespDto, 0)
			}
			topicsByInstances[topic.Instance] = append(topicsByInstances[topic.Instance], topic)
		}
		for instance, topicsWithSameInstance := range topicsByInstances {
			log.DebugC(ctx, "Bulk enriching topics with settings from instance %s", instance)
			if err := helper.bulkEnrichTopicSettings(ctx, topicsWithSameInstance[0].InstanceRef, topicsWithSameInstance); err != nil {
				return err
			}
		}
		return nil
	})
}

// bulkEnrichTopicSettings enriches topics with actual settings. Requires all the topics to belong to the same provided kafkaInstance.
func (helper *HelperImpl) bulkEnrichTopicSettings(ctx context.Context, kafkaInstance *model.KafkaInstance, topics []*model.TopicRegistrationRespDto) error {
	admin, err := helper.createClusterAdmin(ctx, kafkaInstance)
	if err != nil {
		return utils.LogError(log, ctx, "error create admin client to instance: %+v: %w", kafkaInstance, err)
	}
	defer admin.Close()

	topicNames := make([]string, 0, len(topics))
	topicsByName := make(map[string]*model.TopicRegistrationRespDto, len(topics))
	for _, topic := range topics {
		topicNames = append(topicNames, topic.Name)
		topicsByName[topic.Name] = topic
	}

	metadata, err := admin.DescribeTopics(topicNames)
	if err != nil {
		return utils.LogError(log, ctx, "error describe topics: %v: %w", topicNames, err)
	}
	for _, topicMetadata := range metadata {
		if !errors.Is(topicMetadata.Err, sarama.ErrNoError) {
			return utils.LogError(log, ctx, "kafka: failed to get topic's %s metadata: %w", topicMetadata.Name, topicMetadata.Err)
		}
		topic := topicsByName[topicMetadata.Name]

		numPartitions := int32(len(topicMetadata.Partitions))
		topic.ActualSettings.NumPartitions = &numPartitions
		if len(topicMetadata.Partitions) > 0 {
			topic.ActualSettings.ReplicaAssignment = map[int32][]int32{}
			for _, partition := range topicMetadata.Partitions {
				topic.ActualSettings.ReplicaAssignment[partition.ID] = partition.Replicas
			}
			replicationFactor := int16(len(topicMetadata.Partitions[0].Replicas))
			topic.ActualSettings.ReplicationFactor = &replicationFactor
		}

		topicResource := sarama.ConfigResource{
			Type: sarama.TopicResource,
			Name: topic.Name,
		}
		configs, err := admin.DescribeConfig(topicResource)
		if err != nil {
			return utils.LogError(log, ctx, "error getting topic's config for: %v: %w", topic.Name, err)
		}
		topic.ActualSettings.Configs = make(map[string]*string, len(configs))
		for _, configEntry := range configs {
			configValue := configEntry.Value
			topic.ActualSettings.Configs[configEntry.Name] = &configValue
		}
	}
	return nil
}

func IsInstanceAvailable(kafkaInstance *model.KafkaInstance) error {
	healthChecker := NewInstanceHealthChecker(*kafkaInstance, NewHelperImpl(10*time.Second))
	return healthChecker.IsAvailable()
}

func buildTopicDetail(topic *model.TopicRegistration) *sarama.TopicDetail {
	topicDetail := sarama.TopicDetail{
		NumPartitions:     -1,
		ReplicationFactor: -1,
		ReplicaAssignment: nil,
		ConfigEntries:     topic.Configs,
	}
	if !model.IsEmpty(topic.NumPartitions) {
		topicDetail.NumPartitions = *topic.NumPartitions
	}
	if !model.IsEmpty(topic.MinNumPartitions) && model.IsEmpty(topic.NumPartitions) {
		topicDetail.NumPartitions = *topic.MinNumPartitions
	}
	if !model.IsEmpty(topic.ReplicationFactor) {
		topicDetail.ReplicationFactor = *topic.ReplicationFactor
	}
	if topic.ReplicaAssignment != nil {
		topicDetail.ReplicaAssignment = topic.ReplicaAssignment
	}
	return &topicDetail
}

// buildPartitionAssignment transforms replicaAssignment field from update request JSON
// to 2-dimensional array of replica assignments consumed by sarama topics update API.
//
// Parameter affectedPartitionsNum specifies how many partition assignments must be included in result
// (other assignments form replicaAssignment parameters will be ignored).
// It is useful for sarama.ClusterAdmin#CreatePartitions API since this API consumes assignments only for newly added partitions.
func buildPartitionAssignment(affectedPartitionsNum int32, replicaAssignment map[int32][]int32) [][]int32 {
	if len(replicaAssignment) == 0 {
		return [][]int32{}
	}
	totalPartitions := int32(len(replicaAssignment))
	result := make([][]int32, affectedPartitionsNum)

	partitionId := totalPartitions - affectedPartitionsNum
	var idx int32
	for idx = 0; idx < affectedPartitionsNum; idx++ {
		result[idx] = replicaAssignment[partitionId]
		partitionId++
	}
	return result
}

func equalsInt32(first, second *int32) bool {
	if first == second {
		return true
	}
	if first == nil || second == nil {
		return false
	}
	return *first == *second
}

type SaramaLogger struct {
	ctx context.Context
	log logging.Logger
}

func NewSaramaLogger(ctx context.Context, log logging.Logger) *SaramaLogger {
	return &SaramaLogger{ctx, log}
}

func (sl *SaramaLogger) format(v ...interface{}) string {
	switch len(v) {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("%v", v[0])
	default:
		return fmt.Sprintf(v[0].(string), v[1:]...)
	}
}
func (sl *SaramaLogger) Print(v ...interface{}) {
	sl.log.DebugC(sl.ctx, strings.TrimSpace(sl.format(v...)))
}
func (sl *SaramaLogger) Printf(format string, v ...interface{}) {
	sl.log.DebugC(sl.ctx, strings.TrimSpace(fmt.Sprintf(format, v...)))
}
func (sl *SaramaLogger) Println(v ...interface{}) {
	sl.log.DebugC(sl.ctx, sl.format(v...))
}

func newMetricCounter(methodName string) prometheus.Histogram {
	metric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace:   "maas",
		Subsystem:   "kafka",
		Name:        "helper",
		Help:        "",
		ConstLabels: map[string]string{"method": methodName},
		Buckets:     []float64{0.5, 1.0, 5.0},
	})
	if err := prometheus.Register(metric); err != nil {
		log.Errorf("error register kafka helper for: %+v", methodName)
	}
	return metric
}

func measureTime(m prometheus.Histogram, f func() error) error {
	_, err := measureTimeValue(m, func() (any, error) { return nil, f() })
	return err
}

func measureTimeValue[T any](m prometheus.Histogram, f func() (T, error)) (T, error) {
	start := time.Now()
	result, err := f()
	elapsed := time.Since(start)
	m.Observe(float64(elapsed) / float64(time.Second))
	return result, err
}
