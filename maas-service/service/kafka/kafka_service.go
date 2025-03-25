package kafka

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"maas/maas-service/eventbus"
	"maas/maas-service/model"
	"maas/maas-service/monitoring"
	"maas/maas-service/msg"
	"maas/maas-service/service/auth"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/instance"
	"maas/maas-service/service/kafka/helper"
	"maas/maas-service/utils"
	"maas/maas-service/validator"
	"reflect"
	"strings"
	"time"
)

//go:generate mockgen -source=kafka_service.go -destination=kafka_service_mock.go -package kafka
type KafkaDao interface {
	WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error

	FindTopicsBySearchRequest(ctx context.Context, search *model.TopicSearchRequest) ([]*model.TopicRegistration, error)
	InsertTopicRegistration(ctx context.Context, reg *model.TopicRegistration, ext func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error)) (*model.TopicRegistrationRespDto, error)
	UpdateTopicRegistration(ctx context.Context, reg *model.TopicRegistration, ext func(reg *model.TopicRegistration) error) error
	DeleteTopicRegistration(ctx context.Context, registration *model.TopicRegistration, ext func(reg *model.TopicRegistration) error) error

	FindTopicDefinitions(ctx context.Context, search TopicDefinitionSearch) ([]model.TopicDefinition, error)

	UpdateTopicDefinition(ctx context.Context, topicDefinition *model.TopicDefinition) error
	InsertTopicDefinition(ctx context.Context, topicDefinition *model.TopicDefinition) error
	DeleteTopicDefinition(ctx context.Context, classifier *model.Classifier) (*model.TopicDefinition, error)
	DeleteTopicDefinitionsByNamespace(ctx context.Context, namespace string) error

	FindTopicTemplateByNameAndNamespace(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error)
	InsertTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error
	UpdateTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error
	FindTopicsByTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) ([]model.TopicRegistration, error)
	MakeTopicsDirtyByTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error
	FindAllTopicTemplatesByNamespace(ctx context.Context, namespace string) ([]model.TopicTemplate, error)
	DeleteTopicTemplate(ctx context.Context, template *model.TopicTemplate) error
	DeleteTopicTemplatesByNamespace(ctx context.Context, namespace string) error

	Warmup(ctx context.Context, primary string, secondary string) error
	Migrate(ctx context.Context) error
	FindTopicTemplateById(ctx context.Context, id int64) (*model.TopicTemplate, error)
}

type KafkaService interface {
	GetOrCreateTopic(ctx context.Context, topic *model.TopicRegistration, onTopicExists model.OnEntityExistsEnum) (bool, *model.TopicRegistrationRespDto, error)
	GetOrCreateLazyTopic(ctx context.Context, classifier *model.Classifier, onTopicExists model.OnEntityExistsEnum) (bool, *model.TopicRegistrationRespDto, error)
	GetOrCreateTopicWithAuth(ctx context.Context, topic *model.TopicRegistration, onTopicExists model.OnEntityExistsEnum, auth func(ctx context.Context, namespace string, classifier *model.Classifier) error) (bool, *model.TopicRegistrationRespDto, error)
	GetTopicByClassifier(ctx context.Context, classifier model.Classifier) (*model.TopicRegistrationRespDto, error)
	GetTopicByClassifierWithBgDomain(ctx context.Context, classifier model.Classifier) (*model.TopicRegistrationRespDto, error)
	SearchTopics(ctx context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistrationRespDto, error)
	DeleteTopics(ctx context.Context, searchReq *model.TopicSearchRequest) (*model.TopicDeletionResp, error)
	DeleteTopic(ctx context.Context, topic *model.TopicRegistration, leaveRealTopicIntact bool) error
	SyncAllTopicsToKafka(ctx context.Context, namespace string) ([]model.KafkaTopicSyncReport, error)
	SyncTopicToKafka(ctx context.Context, classifier model.Classifier) (*model.KafkaTopicSyncReport, error)

	SearchTopicsInDB(ctx context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistration, error)

	GetOrCreateTopicDefinition(ctx context.Context, topicDefinition *model.TopicDefinition) (bool, *model.TopicDefinition, error)
	GetTopicDefinitionsByNamespaceAndKind(ctx context.Context, namespace string, kind string) ([]model.TopicDefinition, error)
	DeleteTopicDefinition(ctx context.Context, classifier *model.Classifier) (*model.TopicDefinition, error)

	CreateTopicByTenantTopic(ctx context.Context, topicDefinition model.TopicDefinition, tenant model.Tenant) (*model.TopicRegistrationRespDto, error)

	GetTopicTemplateByNameAndNamespace(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error)
	CreateTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) (*model.TopicTemplate, error)
	MakeTopicsDirtyByTemplate(ctx context.Context, template *model.TopicTemplate) error
	UpdateTopicSettingsByTemplate(ctx context.Context, topic model.TopicRegistration, template model.TopicTemplate) (*model.TopicRegistrationRespDto, error)
	UpdateTopicTemplate(ctx context.Context, template model.TopicTemplate, settings model.TopicSettings) error
	GetKafkaTopicTemplatesByNamespace(ctx context.Context, namespace string) ([]model.TopicTemplate, error)
	DeleteTopicTemplate(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error)

	WatchTopicsCreate(ctx context.Context, classifiers []model.Classifier, timeout time.Duration) ([]*model.TopicRegistrationRespDto, error)
	GetDiscrepancyReport(ctx context.Context, namespace string, filter func(item model.DiscrepancyReportItem) bool) ([]model.DiscrepancyReportItem, error)

	MigrateKafka(ctx context.Context) error
	CleanupNamespace(ctx context.Context, namespace string) error
	Warmup(ctx context.Context, state *domain.BGState) error
	Commit(ctx context.Context, state *domain.BGState) error
	Promote(ctx context.Context, state *domain.BGState) error
	Rollback(ctx context.Context, state *domain.BGState) error
	DestroyDomain(ctx context.Context, namespaces *domain.BGNamespaces) error
}

type EventBus interface {
	Broadcast(ctx context.Context, kind string, message string) error
	AddListener(listener eventbus.Listener)
}

type KafkaServiceImpl struct {
	dao             KafkaDao
	instanceService instance.KafkaInstanceService
	helper          helper.Helper
	auditService    monitoring.Auditor
	bgDomainService BGDomainService
	authService     auth.AuthService

	eventbus      EventBus
	topicCreateCh <-chan model.Classifier
}

const eventBus_TopicCreateKind = "topic.create"

type BGDomainService interface {
	FindByNamespace(ctx context.Context, namespace string) (*domain.BGNamespaces, error)
}

func NewKafkaService(dao KafkaDao, instanceService instance.KafkaInstanceService, helper helper.Helper, auditService monitoring.Auditor, bgDomainService BGDomainService, eb EventBus, authService auth.AuthService) *KafkaServiceImpl {
	ch := make(chan model.Classifier)
	if eb != nil {
		eb.AddListener(func(ctx context.Context, event eventbus.Event) {
			if event.Kind != eventBus_TopicCreateKind {
				return // ignore event we are not interested in
			}
			if classifier, err := model.NewClassifierFromReq(event.Message); err == nil {
				ch <- classifier
			} else {
				log.Error("error handle eventbus event: %+v, %w", event, err)
			}
		})
	}

	return &KafkaServiceImpl{
		dao:             dao,
		instanceService: instanceService,
		helper:          helper,
		auditService:    auditService,
		topicCreateCh:   ch,
		bgDomainService: bgDomainService,
		eventbus:        eb,
		authService:     authService,
	}
}

var ErrSearchTopicAttemptWithEmptyCriteria = errors.New("attempt to search with empty search request")

func (srv *KafkaServiceImpl) resolveTopicName(ctx context.Context, nameTemplate string, classifier model.Classifier, versioned bool) (string, error) {
	tenantId := classifier.TenantId
	if nameTemplate == "" {
		// try to use one of default patterns
		if tenantId != "" {
			nameTemplate = "maas.{{namespace}}.{{tenantId}}.{{name}}"
		} else {
			nameTemplate = "maas.{{namespace}}.{{name}}"
		}
	}

	namespace := classifier.Namespace
	if !versioned {
		domain, err := srv.bgDomainService.FindByNamespace(ctx, classifier.Namespace)
		if err != nil {
			return "", fmt.Errorf("error getting bg domain namespaces pair: %w", err)
		}
		if domain != nil {
			namespace = domain.Origin
		}
	}

	topicName := strings.NewReplacer(
		"{{name}}", classifier.Name, "%name%", classifier.Name,
		"{{tenantId}}", tenantId, "%tenantId%", tenantId,
		"{{namespace}}", namespace, "%namespace%", namespace,
	).Replace(nameTemplate)

	const TopicLengthLimit = 249
	if len(topicName) > TopicLengthLimit {
		return "", fmt.Errorf("resolved topic name '%s' is too long. Limit: %v symbols", topicName, TopicLengthLimit)
	}

	return topicName, nil
}

func (srv *KafkaServiceImpl) withTopicRequestAudit(ctx context.Context, handler func() (*model.TopicRegistrationRespDto, error)) (*model.TopicRegistrationRespDto, error) {
	reg, err := handler()
	if reg != nil && err == nil {
		srv.auditService.AddEntityRequestStat(ctx, monitoring.EntityTypeTopic, reg.Name, reg.Instance)
	}
	return reg, err

}

// GetOrCreateTopic creates new topic with the specified name in Kafka instance or returns existing, if topic already exists.
//
// First return value (bool) indicates, that existing topic was found
// (true - existing topic was found; false - new topic was created).
func (srv *KafkaServiceImpl) GetOrCreateTopic(ctx context.Context, topic *model.TopicRegistration, onTopicExists model.OnEntityExistsEnum) (bool, *model.TopicRegistrationRespDto, error) {
	return srv.GetOrCreateTopicWithAuth(ctx, topic, onTopicExists, srv.authService.CheckSecurityForBoundNamespaces)
}

func (srv *KafkaServiceImpl) GetOrCreateTopicWithAuth(ctx context.Context, topic *model.TopicRegistration, onTopicExists model.OnEntityExistsEnum, auth func(ctx context.Context, namespace string, classifier *model.Classifier) error) (bool, *model.TopicRegistrationRespDto, error) {
	found := false
	reg, err := srv.withTopicRequestAudit(ctx, func() (*model.TopicRegistrationRespDto, error) {
		log.InfoC(ctx, "Getting or create topic by classifier: %+v", topic.Classifier)

		if model.SecurityContextOf(ctx).IsCompositeIsolationDisabled() {
			log.InfoC(ctx, "Composite isolation disabled by requester")
		} else {
			err := auth(ctx, model.RequestContextOf(ctx).Namespace, topic.Classifier)
			if err != nil {
				return nil, utils.LogError(log, ctx, "topic access error for classifier %v: %w", topic.Classifier, err)
			}
		}

		var err error
		topic.Topic, err = srv.resolveTopicName(ctx, topic.Topic, *topic.Classifier, topic.Versioned)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error interpolate real topic name by name template: %w", err)
		}

		log.InfoC(ctx, "GetOrCreateTopic for topic: %+v", topic)
		instance, err := srv.resolveKafkaInstance(ctx, *topic.Classifier, topic.Instance, topic.Classifier.Namespace)
		if err != nil {
			return nil, utils.LogError(log, ctx, "failed to resolve kafka instance for topic %v registration: %w", topic.Topic, err)
		}
		if instance == nil {
			return nil, utils.LogError(log, ctx, "kafka instance with id '%s' is not found: %w", topic.Instance, msg.BadRequest)
		}
		topic.Instance = instance.GetId()
		topic.InstanceRef = instance
		var reg *model.TopicRegistrationRespDto = nil
		opError := srv.dao.WithLock(ctx, topic.Classifier.ToJsonString(), func(ctx context.Context) error {
			existingTopic, err := srv.getTopicByClassifier(ctx, *topic.Classifier)
			if err != nil {
				return utils.LogError(log, ctx, "Error in getTopicByClassifierString during GetOrCreateTopic: %w", err)
			}

			if existingTopic != nil && !topic.ExternallyManaged {
				if topic.Instance == "" || topic.Instance == existingTopic.Instance {
					log.InfoC(ctx, "Instance in topic request is empty or equal for existing topic, instance field will be skipped during topic update check for topic with classifier: %v", topic.Classifier)
					result, err := srv.updateTopicSettingsIfNeeded(ctx, existingTopic, topic)
					found = true
					reg = result
					return err
				} else {
					return utils.LogError(log, ctx, "topic with classifier '%s' already exists on '%s' instance: %w", topic.Classifier, existingTopic.Instance, msg.BadRequest)
				}
			}

			err = srv.setDefaultInstanceIfNeeded(ctx, topic)
			if err != nil {
				return utils.LogError(log, ctx, "Error in setDefaultInstanceIfNeeded during GetOrCreateTopic: %v", err)
			}

			log.DebugC(ctx, "Search kafka topic with name %s on kafka instance %s", topic.Topic, topic.Instance)
			onKafka, err := srv.helper.DoesTopicExistOnKafka(ctx, topic.InstanceRef, topic.Topic)
			if err != nil {
				return utils.LogError(log, ctx, "Error in DoesTopicExistOnKafka during GetOrCreateTopic: %w", err)
			}

			topics, err := srv.SearchTopicsInDB(ctx, &model.TopicSearchRequest{Namespace: topic.Classifier.Namespace, Topic: topic.Topic})
			if err != nil {
				return utils.LogError(log, ctx, "Error during SearchTopicsInDB while in GetOrCreateTopic. Error: %w", err)
			}

			var existingTopicByName *model.TopicRegistration

			if len(topics) > 0 {
				log.InfoC(ctx, "Topic was found in db: %+v", topics[0])
				existingTopicByName = topics[0]
			}

			if onKafka && !topic.ExternallyManaged {
				log.DebugC(ctx, "Kafka topic with name %s was found in kafka instance %s ", topic.Topic, topic.Instance)
				switch onTopicExists {
				case model.Merge:
					// insert record in DB + update topic on kafka
					createdTopic, err := srv.dao.InsertTopicRegistration(ctx, topic, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
						topicDto := reg.ToResponseDto()
						log.DebugC(ctx, "topicDto %+v", topicDto)
						return topicDto, srv.helper.UpdateTopicSettings(ctx, topicDto)
					})
					if err != nil {
						if errors.Is(err, ErrTopicAlreadyExists) && existingTopicByName != nil {
							return utils.LogError(log, ctx, "Topic with name '%s' is already assigned to classifier '%v' for instance with name '%v'. "+
								"You can't change existing topic classifier with GetOrCreate request: %w", existingTopicByName.Topic, existingTopicByName.Classifier, existingTopicByName.Instance, msg.Conflict)
						}
						return err
					}
					reg = createdTopic
					srv.eventbus.Broadcast(ctx, eventBus_TopicCreateKind, createdTopic.Classifier.ToJsonString())
					return nil
				case model.Fail:
					return utils.LogError(log, ctx, "Kafka topic with name '%s' was found in kafka instance '%s', parameter 'on-entity-exists' is set to 'fail'. "+
						"If you want to merge this topic with existing one please set parameter value to 'merge': %w", topic.Topic, topic.Instance, msg.Conflict)
				default:
					return utils.LogError(log, ctx, "unsupported value for 'onTopicExists' field. Supported values: %v, %v: %w", model.Merge, model.Fail, msg.BadRequest)
				}
			} else if !onKafka && topic.ExternallyManaged {
				//if topic is not present in kafka return error
				return utils.LogError(log, ctx, "unable to register externally managed topic, because it is not found in kafka by real name `%v': %w", topic.Topic, msg.Gone)
			} else if onKafka && topic.ExternallyManaged && existingTopic != nil {
				//if topic exists in kafka and exists in database return topic
				reg = topic.ToResponseDto()
				return nil
			} else {
				log.DebugC(ctx, "Kafka topic with name %s was not found in kafka instance %s", topic.Topic, topic.Instance)
			}

			//TODO during migration we allow to use base or relative namespaces only for GET operations, later it should be with srv.dao.GetAllowedNamespaces(ctx, namespace)
			classifier, err := model.NewClassifierFromReq(topic.Classifier.ToJsonString())
			if err != nil {
				return utils.LogError(log, ctx, "Classifier must be not null and with 'name' and 'namespace' fields. Namespace should be equal to namespace client can access: %w", msg.BadRequest)
			}
			err = validator.Get().Var(&classifier, validator.ClassifierTag)
			if err != nil {
				return utils.LogError(log, ctx, "invalid classifier '%+v': %v: %w", classifier, err.Error(), msg.BadRequest)
			}

			reg, err = srv.dao.InsertTopicRegistration(ctx, topic, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
				return srv.helper.CreateTopic(ctx, reg)
			})

			if err == nil {
				srv.eventbus.Broadcast(ctx, eventBus_TopicCreateKind, topic.Classifier.ToJsonString())
			}
			return err
		})
		if reg != nil && reg.Template == "" && topic.Template.Valid {
			template, err := srv.dao.FindTopicTemplateById(ctx, topic.Template.Int64)
			if err != nil {
				return nil, err
			}
			reg.Template = template.Name
		}
		return reg, opError
	})

	return found, reg, err
}

func (srv *KafkaServiceImpl) GetOrCreateLazyTopic(ctx context.Context, topicClassifier *model.Classifier, onTopicExists model.OnEntityExistsEnum) (bool, *model.TopicRegistrationRespDto, error) {

	err := srv.authService.CheckSecurityForBoundNamespaces(ctx, model.RequestContextOf(ctx).Namespace, topicClassifier)
	if err != nil {
		log.ErrorC(ctx, "Error in CheckSecurityForBoundNamespaces: %v", err)
		return false, nil, err
	}

	lazyTopics, err := srv.GetTopicDefinitionsByNamespaceAndKind(ctx, topicClassifier.Namespace, model.TopicDefinitionKindLazy)
	if err != nil {
		log.ErrorC(ctx, "Failed to get lazy topics by namespace: %v", err)
		return false, nil, err
	}

	var compatibleLazyTopics []model.TopicDefinition
	for _, lazyTopic := range lazyTopics {
		definedClassifier := lazyTopic.Classifier

		if definedClassifier.Name != "" && definedClassifier.Name != topicClassifier.Name {
			continue
		}

		bgDomain, err := srv.bgDomainService.FindByNamespace(ctx, definedClassifier.Namespace)
		if err != nil {
			return false, nil, utils.LogError(log, ctx, "Failed to get bg domain by namespace: %w", err)
		}
		if definedClassifier.Namespace != "" && (bgDomain != nil && topicClassifier.Namespace != bgDomain.Origin && topicClassifier.Namespace != bgDomain.Peer) {
			continue
		}

		if definedClassifier.TenantId != "" && definedClassifier.TenantId != topicClassifier.TenantId {
			continue
		}

		compatibleLazyTopics = append(compatibleLazyTopics, lazyTopic)
	}
	if len(compatibleLazyTopics) == 0 {
		return false, nil, utils.LogError(log, ctx, "No compatible lazy topic found: %w", msg.BadRequest)
	}
	if len(compatibleLazyTopics) > 1 {
		return false, nil, utils.LogError(log, ctx, "more than one lazy topic found compatible with your classifier: %w", msg.Conflict)
	}
	finalLazyTopic := compatibleLazyTopics[0]
	finalLazyTopic.Classifier = topicClassifier
	topicTemplate, err := srv.GetTopicTemplateByNameAndNamespace(ctx, finalLazyTopic.Template, finalLazyTopic.Classifier.Namespace)
	if err != nil {
		return false, nil, err
	}
	topicRegistration, err := finalLazyTopic.MakeTopicDefinitionResponse().BuildTopicRegFromReq(topicTemplate)
	if err != nil {
		return false, nil, err
	}
	return srv.GetOrCreateTopic(ctx, topicRegistration, onTopicExists)
}

func (srv *KafkaServiceImpl) updateTopicSettingsIfNeeded(ctx context.Context, existingTopic, newTopic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
	if existingTopic.SettingsAreEqual(newTopic) {
		result := existingTopic.ToResponseDto()
		if err := srv.helper.GetTopicSettings(ctx, result); err != nil {
			log.ErrorC(ctx, "Failed to load existing topic %s settings from kafka instance %s: %v", newTopic.Topic, newTopic.Instance, err)
			return nil, err
		}
		return result, nil
	} else {
		existingTopic.Template = newTopic.Template
		existingTopic.CreateReq = newTopic.CreateReq
		return srv.updateSettings(ctx, existingTopic, &newTopic.TopicSettings)
	}
}

func (srv *KafkaServiceImpl) updateSettings(ctx context.Context, existingTopic *model.TopicRegistration, newSettings *model.TopicSettings) (*model.TopicRegistrationRespDto, error) {
	if !model.IsEmpty(newSettings.NumPartitions) && !model.IsEmpty(newSettings.MinNumPartitions) {
		return nil, utils.LogError(log, ctx, "error update settings: `numPartition' property is mutual exclusive with `minNumPartition': %w", msg.BadRequest)
	}

	existingTopic.NumPartitions = newSettings.NumPartitions
	existingTopic.MinNumPartitions = newSettings.MinNumPartitions
	existingTopic.ReplicationFactor = newSettings.ReplicationFactor
	existingTopic.ReplicaAssignment = newSettings.ReplicaAssignment
	existingTopic.Configs = newSettings.Configs
	existingTopic.Versioned = newSettings.Versioned

	result := existingTopic.ToResponseDto()
	log.DebugC(ctx, "topic before UpdateTopicRegistration in updateTopicSettingsIfNeeded: %+v", result)
	if err := srv.dao.UpdateTopicRegistration(ctx, existingTopic, func(reg *model.TopicRegistration) error {
		if err := srv.helper.UpdateTopicSettings(ctx, result); err != nil {
			log.ErrorC(ctx, "Failed to update existing topic %s settings in kafka instance %s: %v", existingTopic.Topic, existingTopic.Instance, err)
			return err
		} else {
			return nil
		}
	}); err != nil {
		log.ErrorC(ctx, "Failed to update topic %s settings in database due to error: %v", existingTopic.Topic, err)
		return nil, err
	}
	return result, nil
}

func (srv *KafkaServiceImpl) GetTopicByClassifier(ctx context.Context, classifier model.Classifier) (*model.TopicRegistrationRespDto, error) {
	return srv.withTopicRequestAudit(ctx, func() (*model.TopicRegistrationRespDto, error) {
		log.InfoC(ctx, "Getting topic by classifier: %+v", classifier)

		if model.SecurityContextOf(ctx).IsCompositeIsolationDisabled() {
			log.InfoC(ctx, "Composite isolation disabled by requester")
		} else {
			err := srv.authService.CheckSecurityForBoundNamespaces(ctx, model.RequestContextOf(ctx).Namespace, &classifier)
			if err != nil {
				return nil, utils.LogError(log, ctx, "topic access error for classifier %v: %w", classifier, err)
			}
		}

		topic, err := srv.getTopicByClassifier(ctx, classifier)
		if err != nil {
			log.ErrorC(ctx, "Error while getting topic by classifier from db: %v", err)
			return nil, err
		}
		if topic == nil {
			log.InfoC(ctx, "Topic not found with classifier: %+v", classifier)
			return nil, nil
		}

		result := topic.ToResponseDto()
		if err := srv.helper.GetTopicSettings(ctx, result); err != nil {
			log.ErrorC(ctx, "Failed to load existing topic %s settings from kafka instance %s: %v", topic.Topic, topic.Instance, err)
			return nil, err
		}
		return result, nil
	})
}

func (srv *KafkaServiceImpl) GetTopicByClassifierWithBgDomain(ctx context.Context, classifier model.Classifier) (*model.TopicRegistrationRespDto, error) {
	bgNamespaces, err := srv.bgDomainService.FindByNamespace(ctx, classifier.Namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "Error occurred during FindByNamespace while in GetTopicByClassifierWithBgDomain: '%w'", err)
	}

	if bgNamespaces != nil && classifier.Namespace == bgNamespaces.ControllerNamespace {
		classifier.Namespace = bgNamespaces.Origin
	}

	return srv.GetTopicByClassifier(ctx, classifier)
}

func (srv *KafkaServiceImpl) getTopicByClassifier(ctx context.Context, classifier model.Classifier) (*model.TopicRegistration, error) {
	topics, err := srv.dao.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Classifier: classifier})
	if err != nil {
		return nil, utils.LogError(log, ctx, "failed to load topic by classifier %s from db: %w", classifier, err)
	}
	if topics == nil || len(topics) == 0 {
		return nil, nil
	}

	// there is can be only one topic by classifier
	topic := topics[0]

	if topic.InstanceRef, err = srv.instanceService.GetById(ctx, topic.Instance); err != nil {
		return nil, utils.LogError(log, ctx, "failed to load kafka instance for topic %v registration: %w", topic, err)
	}

	return topic, nil
}

func (srv *KafkaServiceImpl) SearchTopics(ctx context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistrationRespDto, error) {
	topics, err := srv.SearchTopicsInDB(ctx, searchReq)
	if err != nil {
		log.ErrorC(ctx, "Error occurred while searching for topics in database: %v", err)
		return nil, err
	}
	if len(topics) == 0 {
		return []*model.TopicRegistrationRespDto{}, nil
	}

	result := make([]*model.TopicRegistrationRespDto, 0, len(topics))
	for _, topic := range topics {
		result = append(result, topic.ToResponseDto())
	}
	if err := srv.helper.BulkGetTopicSettings(ctx, result); err != nil {
		return nil, utils.LogError(log, ctx, "Failed to load existing topics settings from kafka: %w", err)
	}
	return result, nil
}

// searchTopicsInternal searches topics in database but does not enrich settings with actual (effective) values from Kafka.
func (srv *KafkaServiceImpl) SearchTopicsInDB(ctx context.Context, searchReq *model.TopicSearchRequest) ([]*model.TopicRegistration, error) {
	if searchReq.IsEmpty() {
		return nil, ErrSearchTopicAttemptWithEmptyCriteria
	}
	topics, err := srv.dao.FindTopicsBySearchRequest(ctx, searchReq)
	if err != nil {
		return nil, utils.LogError(log, ctx, "Error occurred while searching for topics in database: %w", err)
	}
	if len(topics) == 0 {
		return nil, nil
	}

	// enrich found topics with instance values
	instances := make(map[string]*model.KafkaInstance)
	for _, topic := range topics {
		// cache kafka instances locally to avoid extra database calls
		if _, found := instances[topic.Instance]; !found {
			inst, err := srv.instanceService.GetById(ctx, topic.Instance)
			if err != nil {
				return nil, utils.LogError(log, ctx, "error getting instance by id: %w", err)
			}
			instances[topic.Instance] = inst
		}
		topic.InstanceRef = instances[topic.Instance]
	}
	return topics, nil
}

func (srv *KafkaServiceImpl) DeleteTopics(ctx context.Context, searchReq *model.TopicSearchRequest) (*model.TopicDeletionResp, error) {
	log.InfoC(ctx, "Remove kafka topics by search request '%+v'", searchReq)

	topics, err := srv.SearchTopicsInDB(ctx, searchReq)
	if err != nil {
		return nil, err
	}
	response := new(model.TopicDeletionResp)

	for _, topic := range topics {
		if err = srv.DeleteTopic(ctx, topic, searchReq.LeaveRealTopicIntact); err != nil {
			log.ErrorC(ctx, "Error deleting kafka topic %s: %v", topic, err)
			response.FailedToDelete = append(response.FailedToDelete, model.TopicDeletionError{
				Topic:   topic.ToResponseDto(),
				Message: err.Error(),
			})
		} else {
			response.DeletedSuccessfully = append(response.DeletedSuccessfully, *topic.ToResponseDto())
		}
	}
	return response, nil
}

func (srv *KafkaServiceImpl) DeleteTopic(ctx context.Context, topic *model.TopicRegistration, leaveRealTopicIntact bool) error {
	return srv.dao.DeleteTopicRegistration(ctx, topic, func(reg *model.TopicRegistration) error {
		if leaveRealTopicIntact == false {
			return srv.helper.DeleteTopic(ctx, topic)
		}
		return nil
	})
}

func (srv *KafkaServiceImpl) SyncAllTopicsToKafka(ctx context.Context, namespace string) ([]model.KafkaTopicSyncReport, error) {
	syncReport := make([]model.KafkaTopicSyncReport, 0)
	log.InfoC(ctx, "start kafka topics reconciliation")
	topics, err := srv.dao.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: namespace})
	if err != nil {
		return nil, err
	}
	for _, topic := range topics {
		topic.InstanceRef, err = srv.instanceService.GetById(ctx, topic.Instance)
		if err != nil {
			log.ErrorC(ctx, "can not resolve kafka '%s' instance for '%s' topic; skip it: %s", topic.Instance, topic.Topic, err.Error())
			syncReport = append(syncReport, *newErrKafkaTopicSyncReport(topic, err))
			continue
		}
		onKafka, err := srv.helper.DoesTopicExistOnKafka(ctx, topic.InstanceRef, topic.Topic)
		if err != nil {
			log.ErrorC(ctx, "can not check does '%s' topic exists on kafka; skip it: %s", topic.Topic, err.Error())
			syncReport = append(syncReport, *newErrKafkaTopicSyncReport(topic, err))
			continue
		}
		if !onKafka {
			_, err := srv.helper.CreateTopic(ctx, topic)
			if err != nil {
				log.ErrorC(ctx, "can not create kafka topic '%s' during reconciliation: %s", topic.Topic, err)
				syncReport = append(syncReport, *newErrKafkaTopicSyncReport(topic, err))
				continue
			}
			syncReport = append(syncReport, *newAddedKafkaTopicSyncReport(topic))
			continue
		} else {
			syncReport = append(syncReport, *newExistsKafkaTopicSyncReport(topic))
			continue
		}
	}
	log.InfoC(ctx, "kafka topics reconciliation has been finished")
	return syncReport, nil
}

func (srv *KafkaServiceImpl) SyncTopicToKafka(ctx context.Context, classifier model.Classifier) (*model.KafkaTopicSyncReport, error) {
	log.InfoC(ctx, "start kafka topic reconciliation by classifier: %v", classifier)

	err := srv.authService.CheckSecurityForSingleNamespace(ctx, model.RequestContextOf(ctx).Namespace, &classifier)
	if err != nil {
		log.ErrorC(ctx, "Error in CheckSecurityForSingleNamespace: %v", err)
		return nil, err
	}

	topics, err := srv.dao.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Classifier: classifier})
	if err != nil {
		return nil, err
	}

	if len(topics) > 1 {
		return nil, utils.LogError(log, ctx, "logical error: more then one topic was found in MaaS by classifier, classifier must be unique: %v", classifier)
	}

	if len(topics) == 0 {
		log.WarnC(ctx, "topic was not found in MaaS by classifier: %v", classifier)

		return &model.KafkaTopicSyncReport{
			Name:       "",
			Classifier: classifier,
			Status:     model.SyncStatusNotFound,
			ErrMsg:     "",
		}, nil
	}

	topic := topics[0]

	topic.InstanceRef, err = srv.instanceService.GetById(ctx, topic.Instance)
	if err != nil {
		return nil, utils.LogError(log, ctx, "can not resolve kafka '%s' instance for '%s' topic; error: %w", topic.Instance, topic.Topic, err)
	}

	onKafka, err := srv.helper.DoesTopicExistOnKafka(ctx, topic.InstanceRef, topic.Topic)
	if err != nil {
		return nil, utils.LogError(log, ctx, "can not check does '%s' topic exists on kafka; error: %w", topic.Topic, err)
	}

	if onKafka {
		log.InfoC(ctx, "Topic already exists in kafka, skip it; topic: %v", topic.Topic)
		return newExistsKafkaTopicSyncReport(topic), nil
	}

	_, err = srv.helper.CreateTopic(ctx, topic)
	if err != nil {
		return nil, utils.LogError(log, ctx, "can not create kafka topic '%s' during reconciliation: %w", topic.Topic, err)
	}
	log.InfoC(ctx, "Topic was successfully synced in kafka; topic: %v", topic.Topic)
	return newAddedKafkaTopicSyncReport(topic), nil
}

func newErrKafkaTopicSyncReport(topic *model.TopicRegistration, err error) *model.KafkaTopicSyncReport {
	return newKafkaTopicSyncReport(topic, err, model.SyncStatusError)
}

func newAddedKafkaTopicSyncReport(topic *model.TopicRegistration) *model.KafkaTopicSyncReport {
	return newKafkaTopicSyncReport(topic, nil, model.SyncStatusAdded)
}

func newExistsKafkaTopicSyncReport(topic *model.TopicRegistration) *model.KafkaTopicSyncReport {
	return newKafkaTopicSyncReport(topic, nil, model.SyncStatusExists)
}

func newKafkaTopicSyncReport(topic *model.TopicRegistration, err error, status string) *model.KafkaTopicSyncReport {
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	return &model.KafkaTopicSyncReport{
		Name:       topic.Topic,
		Classifier: *topic.Classifier,
		Status:     status,
		ErrMsg:     errMsg,
	}
}

func (srv *KafkaServiceImpl) resolveKafkaInstance(ctx context.Context, classifier model.Classifier, reqInstance string, namespace string) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Starting to resolve kafka instance for topic with classifier '%v'", classifier)
	designator, err := srv.instanceService.GetKafkaInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		log.ErrorC(ctx, "error during getting kafka instance designator for namespace '%+v': %v", namespace, err)
		return nil, err
	}

	if designator != nil {
		log.InfoC(ctx, "instance will be resolved by instance designator found for namespace '%+v', designator: '%v'", namespace, designator)

		if reqInstance != "" {
			return nil, utils.LogError(log, ctx, "found conflicted configuration values: Kafka instance designator defined for namespace '%v' and topic declaration contains field instanceId='%v'.  "+
				"Field instanceId still supported but is deprecated. Please migrate on designators: %w", namespace, reqInstance, msg.Conflict)
		}

		if err != nil {
			log.ErrorC(ctx, "error during ConvertToClassifier for topic classifier '%v': %v", classifier, err)
			return nil, err
		}

		instance, match, matchedBy := srv.MatchDesignator(ctx, classifier, designator)
		if match {
			log.InfoC(ctx, "Selector with ClassifierMatch '%+v' from instance designator was found for topic with classifier '%+v'. Chosen instance is '%+v'", matchedBy, classifier, instance)
		}

		if instance == nil && designator.DefaultInstanceId != nil {
			instance = designator.DefaultInstance
			log.InfoC(ctx, "no selector from instance designator was found for topic with classifier '%s'. Default designator instance will be used :'%+v'", classifier, instance)
		}

		if instance == nil {
			log.InfoC(ctx, "Instance designator exists, but specified selectors didn't match and value for default designator instance id is empty. Instance resolved as ''(empty) for topic with classifier: %v", classifier)
			defaultInstance, err := srv.instanceService.GetDefault(ctx)
			if err != nil {
				return nil, utils.LogError(log, ctx, "error getting default instance: %w", err)
			}
			if defaultInstance == nil {
				return nil, utils.LogError(log, ctx, "no default kafka instance defined yet: %w", msg.Conflict)
			}
			return defaultInstance, nil
		} else {
			log.InfoC(ctx, "Resolved kafka instance by instance designator for topic with classifier '%+v' is '%+v'", classifier, instance)
			return instance, nil
		}
	} else {
		log.InfoC(ctx, "Instance will be resolved by instance field in topic (or default), because instance designator was not found. topic instance field: '%v'", reqInstance)

		if reqInstance != "" {
			instance, err := srv.instanceService.GetById(ctx, reqInstance)
			if err != nil {
				return nil, utils.LogError(log, ctx, "unexpected error getting instance by id: %w", err)
			}
			return instance, nil
		}

		defaultInstance, err := srv.instanceService.GetDefault(ctx)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error getting default instance: %w", err)
		}
		if defaultInstance == nil {
			return nil, utils.LogError(log, ctx, "no default instance found: %w", msg.Conflict)
		}
		return defaultInstance, nil
	}
}

func (srv *KafkaServiceImpl) setDefaultInstanceIfNeeded(ctx context.Context, topic *model.TopicRegistration) error {
	log.InfoC(ctx, "Starting to resolve kafka instance for topic with classifier '%v'", topic.Classifier)

	if topic.Instance == "" {
		instance, err := srv.instanceService.GetDefault(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "error getting default instance: %w", err)
		}
		if instance == nil {
			return utils.LogError(log, ctx, "no default Kafka instance is set: %w", msg.NotFound)
		}

		topic.Instance = instance.Id
		topic.InstanceRef = instance
		log.InfoC(ctx, "Topic instance field was empty, it is set to default MaaS Kafka instance '%v' for topic with classifier '%v'", topic.Instance, topic.Classifier)
		return nil
	} else {
		log.InfoC(ctx, "Topic instance field is already not empty, but equals '%v' for topic with classifier '%v'", topic.Instance, topic.Classifier)
		return nil
	}
}

//TopicTemplate section

func (srv *KafkaServiceImpl) GetTopicTemplateByNameAndNamespace(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error) {
	return srv.dao.FindTopicTemplateByNameAndNamespace(ctx, name, namespace)
}

func (srv *KafkaServiceImpl) CreateTopicTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) (*model.TopicTemplate, error) {
	err := srv.dao.InsertTopicTemplate(ctx, topicTemplate)
	if err != nil {
		return nil, err
	}
	return topicTemplate, nil
}

func (srv *KafkaServiceImpl) MakeTopicsDirtyByTemplate(ctx context.Context, topicTemplate *model.TopicTemplate) error {
	err := srv.dao.MakeTopicsDirtyByTemplate(ctx, topicTemplate)
	if err != nil {
		return err
	}
	return nil
}

func (srv *KafkaServiceImpl) UpdateTopicSettingsByTemplate(ctx context.Context, existingTopic model.TopicRegistration, template model.TopicTemplate) (*model.TopicRegistrationRespDto, error) {
	existingTopic.Dirty = false
	return srv.updateSettings(ctx, &existingTopic, &template.TopicSettings)
}

func (srv *KafkaServiceImpl) UpdateTopicTemplate(ctx context.Context, template model.TopicTemplate, settings model.TopicSettings) error {
	template.NumPartitions = settings.NumPartitions
	template.MinNumPartitions = settings.MinNumPartitions
	template.ReplicationFactor = settings.ReplicationFactor
	template.ReplicaAssignment = settings.ReplicaAssignment
	template.Configs = settings.Configs

	log.DebugC(ctx, "template before UpdateTopicTemplate: %+v", template)
	err := srv.dao.UpdateTopicTemplate(ctx, &template)
	if err != nil {
		log.ErrorC(ctx, "Failed to update topic template %s settings in database due to error: %v", template, err)
	}
	return err
}

func (srv *KafkaServiceImpl) GetTopicTemplateByName(ctx context.Context, templateName string, namespace string) (*model.TopicTemplate, error) {
	log.InfoC(ctx, "Search topic template name by name: `%v' in namespace: %v", templateName, namespace)
	topicTemplate, err := srv.GetTopicTemplateByNameAndNamespace(ctx, templateName, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error getting topic template: %w", err)
	}
	return topicTemplate, nil
}

func (srv *KafkaServiceImpl) GetKafkaTopicTemplatesByNamespace(ctx context.Context, namespace string) ([]model.TopicTemplate, error) {
	log.InfoC(ctx, "Getting all topic templates by namespace %v", namespace)
	templates, err := srv.dao.FindAllTopicTemplatesByNamespace(ctx, namespace)

	if err != nil {
		return nil, utils.LogError(log, ctx, "error getting all topic templates by namespace `%v': %w", namespace, err)
	}
	return templates, err
}

func (srv *KafkaServiceImpl) DeleteTopicTemplate(ctx context.Context, name string, namespace string) (*model.TopicTemplate, error) {
	log.InfoC(ctx, "Trying to delete topic template by name %v and namespace %v", name, namespace)
	topicTemplate, err := srv.GetTopicTemplateByNameAndNamespace(ctx, name, namespace)
	if err != nil {
		log.ErrorC(ctx, "error getting topic template with name: %v, %v", name, err)
		return nil, err
	}
	if topicTemplate == nil {
		log.WarnC(ctx, "not found topic template with name: %v, %v", name, err)
		return nil, nil
	}

	err = srv.dao.DeleteTopicTemplate(ctx, topicTemplate)
	if err != nil {
		log.ErrorC(ctx, "error deleting topic template with name: %v, %v", name, err)
		return nil, err
	}
	return topicTemplate, err
}

// TopicDefinition section
func (srv *KafkaServiceImpl) GetOrCreateTopicDefinition(ctx context.Context, topicDefinition *model.TopicDefinition) (bool, *model.TopicDefinition, error) {
	log.InfoC(ctx, "Get or create topic definition: %+v", topicDefinition)

	err := validator.Get().Var(topicDefinition.Classifier, validator.ClassifierTag)
	if err != nil {
		return false, nil, utils.LogError(log, ctx, "invalid classifier '%+v': %v: %w", topicDefinition.Classifier, err.Error(), msg.BadRequest)
	}

	clause := TopicDefinitionSearch{Classifier: topicDefinition.Classifier, Kind: topicDefinition.Kind}
	topicDefs, err := srv.dao.FindTopicDefinitions(ctx, clause)
	if err != nil {
		return false, nil, utils.LogError(log, ctx, "error getting topic defintion: %w", err)
	}

	var freshTopic *model.TopicDefinition
	if len(topicDefs) == 1 {
		freshTopic = &topicDefs[0]
	}

	if freshTopic != nil {
		log.InfoC(ctx, "update existing topic defintion")
		freshTopic.Template = topicDefinition.Template
		freshTopic.NumPartitions = topicDefinition.NumPartitions
		freshTopic.MinNumPartitions = topicDefinition.MinNumPartitions
		freshTopic.ReplicaAssignment = topicDefinition.ReplicaAssignment
		freshTopic.ReplicationFactor = topicDefinition.ReplicationFactor
		freshTopic.Configs = topicDefinition.Configs
		err := srv.dao.UpdateTopicDefinition(ctx, freshTopic)
		if err != nil {
			return false, freshTopic, utils.LogError(log, ctx, "failed to update topicDefinition %v in db: %w", topicDefinition, err)
		}
		return true, freshTopic, nil
	} else {
		log.InfoC(ctx, "create new topic defintion")

		//TODO during migration we allow to use base or relative namespaces only for GET operations, later it should be with srv.dao.GetAllowedNamespaces(ctx, namespace)
		namespace := model.RequestContextOf(ctx).Namespace
		err = topicDefinition.Classifier.CheckAllowedNamespaces(namespace)
		if err != nil {
			return false, nil, utils.LogError(log, ctx, "error in VerifyAuth of classifier during GetOrCreateTopicDefinition: %w", err)
		}

		err = srv.dao.InsertTopicDefinition(ctx, topicDefinition)
		if err != nil {
			log.ErrorC(ctx, "Failed to insert topicDefinition %v to db: %v", topicDefinition, err)
			return false, topicDefinition, err
		}
		return false, topicDefinition, nil
	}
}

func (srv *KafkaServiceImpl) GetTopicDefinitionsByNamespaceAndKind(ctx context.Context, namespace string, kind string) ([]model.TopicDefinition, error) {
	log.InfoC(ctx, "Getting all topic definitions by namespace %v and kind %v", namespace, kind)
	topics, err := srv.dao.FindTopicDefinitions(ctx, TopicDefinitionSearch{Namespace: namespace, Kind: kind})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error getting topic definitions by namespace `%v' and kind `%v': %w", namespace, kind, err)
	}
	return topics, err
}

func (srv *KafkaServiceImpl) DeleteTopicDefinition(ctx context.Context, classifier *model.Classifier) (*model.TopicDefinition, error) {
	topicDefinition, err := srv.dao.DeleteTopicDefinition(ctx, classifier)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error deleting topic definition by classifier: %v, %w", classifier, err)
	}
	return topicDefinition, err
}

func (srv *KafkaServiceImpl) CreateTopicByTenantTopic(ctx context.Context, topicDefinition model.TopicDefinition, tenant model.Tenant) (*model.TopicRegistrationRespDto, error) {
	log.InfoC(ctx, "Creating topic by tenant-topic for tenant: %v", tenant)

	topicClassifier := model.Classifier{
		Name:      topicDefinition.Classifier.Name,
		Namespace: topicDefinition.Classifier.Namespace,
		TenantId:  tenant.ExternalId,
	}
	err := validator.Get().Var(&topicClassifier, validator.ClassifierTag)
	if err != nil {
		return nil, utils.LogError(log, ctx, "invalid classifier '%+v': %v: %w", topicClassifier, err.Error(), msg.BadRequest)
	}
	var tenantNullable sql.NullInt64
	if tenant.Id == 0 {
		tenantNullable = sql.NullInt64{Int64: int64(tenant.Id), Valid: false}
	} else {
		tenantNullable = sql.NullInt64{Int64: int64(tenant.Id), Valid: true}
	}

	kafkaInstance, err := srv.instanceService.GetById(ctx, topicDefinition.Instance)
	if err != nil {
		log.ErrorC(ctx, "error get kafka instance by id: %s, err: %v", topicDefinition.Instance, err)
		return nil, err
	}

	topicTemplate, err := srv.GetTopicTemplateByName(ctx, topicDefinition.Template, topicDefinition.Namespace)
	if err != nil {
		log.ErrorC(ctx, "error get topic template by name: %s, err: %v", topicDefinition.Template, err)
		return nil, err
	}

	var topicTemplateNullable sql.NullInt64
	if topicTemplate == nil || topicTemplate.Id == 0 {
		topicTemplateNullable = sql.NullInt64{Int64: int64(0), Valid: false}
	} else {
		topicTemplateNullable = sql.NullInt64{Int64: int64(topicTemplate.Id), Valid: true}
	}

	numPartitions := int32(topicDefinition.NumPartitions)
	minNumPartitions := int32(topicDefinition.MinNumPartitions)
	replicationFactor := int16(topicDefinition.ReplicationFactor)

	newTopicName, err := srv.resolveTopicName(ctx, topicDefinition.Name, topicClassifier, topicDefinition.Versioned)
	if err != nil {
		return nil, err
	}

	topicJson, err := json.Marshal(topicDefinition.MakeTopicDefinitionResponse())
	if err != nil {
		return nil, err
	}

	var topicSettings model.TopicSettings
	if topicTemplate != nil {
		topicSettings = *topicTemplate.GetSettings()
	} else {
		topicSettings = model.TopicSettings{
			NumPartitions:     &numPartitions,
			MinNumPartitions:  &minNumPartitions,
			ReplicationFactor: &replicationFactor,
			ReplicaAssignment: topicDefinition.ReplicaAssignment,
			Configs:           topicDefinition.Configs,
			Versioned:         topicDefinition.Versioned,
		}
	}

	topicRegistration := model.TopicRegistration{
		Classifier:    &topicClassifier,
		Topic:         newTopicName,
		Instance:      topicDefinition.Instance,
		Namespace:     topicDefinition.Namespace,
		TopicSettings: topicSettings,
		Template:      topicTemplateNullable,
		TenantId:      tenantNullable,
		CreateReq:     string(topicJson),
		InstanceRef:   kafkaInstance,
	}
	//Merge if previous operation has failed or topic exists in kafka
	_, topicResp, err := srv.GetOrCreateTopic(ctx, &topicRegistration, model.Merge)
	if err != nil {
		log.ErrorC(ctx, "error during get or create topic while in CreateTopicByTenantTopic: %v, err: %v", topicRegistration, err)
		return nil, err
	}

	return topicResp, nil
}

func (srv *KafkaServiceImpl) CleanupNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "cleanup kafka topics for namespace: %v", namespace)
	searchCriteria := model.TopicSearchRequest{Namespace: namespace}
	responses, err := srv.DeleteTopics(ctx, &searchCriteria)
	if err != nil {
		return utils.LogError(log, ctx, "error deleting topics by namespace: %w", err)
	}

	if len(responses.FailedToDelete) > 0 {
		return utils.LogError(log, ctx, "some topics have failed to delete: %+v", responses.FailedToDelete)
	}

	log.InfoC(ctx, "deleting topic-templates for namespace: %v", namespace)
	if err = srv.dao.DeleteTopicTemplatesByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error deleting topic-templates by namespace: %w", err)
	}

	//lazy-topics, tenant-topics
	log.InfoC(ctx, "deleting lazy and tenant topics for namespace: %v", namespace)
	if err = srv.dao.DeleteTopicDefinitionsByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error deleting lazy-topics, tenant-topics by namespace: %w", err)
	}

	if err := srv.instanceService.DeleteKafkaInstanceDesignatorByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error delete instance designators: %w", err)
	}

	return nil
}

// WatchTopicsCreate
// search topics by given classifier in registrations database or wait any will be created
// Note! do not use this  watch for tenant topics, because tenant topic may be rolled back if tenant rollout process failed
// maximum allowed requestTimeout is 120sec
func (srv *KafkaServiceImpl) WatchTopicsCreate(ctx context.Context, classifiers []model.Classifier, requestTimeout time.Duration) ([]*model.TopicRegistrationRespDto, error) {
	if requestTimeout > 120*time.Second {
		log.WarnC(ctx, "timeout interval `%s' exceeds maximum allowed. Overridden to 2min.", requestTimeout)
		requestTimeout = 120 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	for _, classifier := range classifiers {
		if classifier.TenantId != "" {
			return nil, fmt.Errorf("this API is not suitable to watch tenant topics: %+v", classifier)
		}
	}

	watchLog.InfoC(ctx, "Start watch for topics: %+v, timeout: %v", classifiers, requestTimeout)
	found := make([]*model.TopicRegistrationRespDto, 0)

	for _, classifier := range classifiers {
		dto, err := srv.GetTopicByClassifierWithBgDomain(ctx, classifier)
		if err != nil {
			return nil, utils.LogError(watchLog, ctx, "error search topic by classifier: %v: %w", classifier, err)
		}
		if dto != nil {
			watchLog.DebugC(ctx, "found topic: %+v", dto.Classifier)
			found = append(found, dto)
		}
	}
	if len(found) > 0 {
		watchLog.InfoC(ctx, "return already inserted topics: %v", len(found))
		return found, nil
	}

	watchLog.DebugC(ctx, "Start listen events for topic create")
	for {
		select {
		case <-ctx.Done():
			log.InfoC(ctx, "Topic create watch cancelled by timeout")
			return []*model.TopicRegistrationRespDto{}, nil
		case inserted := <-srv.topicCreateCh:
			log.DebugC(ctx, "handle kafka topic insert event for classifier: %+v", inserted)
			// filter out and revalidate event
			for _, classifier := range classifiers {
				if reflect.DeepEqual(classifier, inserted) {
					watchLog.DebugC(ctx, "event classifier match one of the watched: %+v", inserted)
					dto, err := srv.GetTopicByClassifierWithBgDomain(ctx, classifier)
					if err != nil {
						return nil, utils.LogError(watchLog, ctx, "error search topic by classifier: %+v: %w", classifier, err)
					}
					watchLog.InfoC(ctx, "Return just created topic: %+v", classifier)
					return []*model.TopicRegistrationRespDto{dto}, nil
				}
			}
		}
	}
}

func (srv *KafkaServiceImpl) GetDiscrepancyReport(ctx context.Context, namespace string, filter func(item model.DiscrepancyReportItem) bool) ([]model.DiscrepancyReportItem, error) {
	topicsInDb, err := srv.SearchTopicsInDB(ctx, &model.TopicSearchRequest{
		Namespace: namespace,
	})
	if err != nil {
		return nil, err
	}

	discrepancyReport := make([]model.DiscrepancyReportItem, len(topicsInDb))
	for i, topic := range topicsInDb {
		discrepancyReport[i].Name = topic.Topic
		discrepancyReport[i].Classifier = *topic.Classifier

		topicExistOnKafka, err := srv.helper.DoesTopicExistOnKafka(ctx, topic.InstanceRef, topic.Topic)
		if err != nil {
			return nil, err
		}
		if topicExistOnKafka {
			discrepancyReport[i].Status = model.StatusOk
		} else {
			discrepancyReport[i].Status = model.StatusAbsent
		}
	}
	return filterItems(discrepancyReport, filter), nil
}

func (srv *KafkaServiceImpl) MatchDesignator(ctx context.Context, classifier model.Classifier, designator *model.InstanceDesignatorKafka) (*model.KafkaInstance, bool, model.ClassifierMatch) {
	instance, res, err := instance.MatchDesignator(ctx, classifier, designator, func(namespace string) string {
		domainNamespace, err := srv.bgDomainService.FindByNamespace(ctx, namespace)
		if err != nil {
			log.ErrorC(ctx, "can not get domain namespace for '%s': %w", namespace, err)
			return namespace
		}
		if domainNamespace == nil {
			log.ErrorC(ctx, "no domain namespace for '%s' has been found", namespace)
			return namespace
		}
		return domainNamespace.Origin
	})
	if instance != nil {
		return instance.(*model.KafkaInstance), res, err

	}
	return nil, false, model.ClassifierMatch{}
}

func filterItems(items []model.DiscrepancyReportItem, fits func(item model.DiscrepancyReportItem) bool) []model.DiscrepancyReportItem {
	if fits == nil {
		return items
	}
	filtered := make([]model.DiscrepancyReportItem, 0)
	for _, ri := range items {
		if fits(ri) {
			filtered = append(filtered, ri)
		}
	}
	return filtered
}

func (srv *KafkaServiceImpl) Warmup(ctx context.Context, state *domain.BGState) error {
	return wlog(ctx, "warmup kafka topics", func() error {
		err := srv.dao.Warmup(ctx, state.Origin.Name, state.Peer.Name)
		if err != nil {
			return err
		}
		copyFromNamespace := state.GetActiveNamespace()
		copyToNamespace := state.GetNotActiveNamespace()
		log.InfoC(ctx, "going to copy versioned topics from %s to %s", copyFromNamespace, copyToNamespace)
		err = srv.copyVersionedTopics(ctx, copyFromNamespace.Name, copyToNamespace.Name)
		if err != nil {
			return err
		}

		return nil
	})
}

func (srv *KafkaServiceImpl) Commit(ctx context.Context, state *domain.BGState) error {
	return wlog(ctx, "commit kafka topics", func() error {
		trueVal := true
		notActiveNamespace := state.GetNotActiveNamespace()
		log.InfoC(ctx, "going to delete versioned topics from %s namespace", notActiveNamespace)
		_, err := srv.DeleteTopics(ctx, &model.TopicSearchRequest{
			Namespace: notActiveNamespace.Name,
			Versioned: &trueVal,
		})
		return err
	})
}

func (srv *KafkaServiceImpl) Promote(ctx context.Context, state *domain.BGState) error {
	return nil
}

func (srv *KafkaServiceImpl) Rollback(ctx context.Context, state *domain.BGState) error {
	return nil
}

// DestroyDomain just simple sanity check, because there are also may exist other entities such: lazy-topics, templates, designators, etc
func (srv *KafkaServiceImpl) DestroyDomain(ctx context.Context, namespaces *domain.BGNamespaces) error {
	check := func(ns string) error {
		if topics, err := srv.dao.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Namespace: ns}); err == nil {
			if len(topics) > 0 {
				return utils.LogError(log, ctx, "namespace is not empty: found topics in namespace `%v': %w", ns, msg.Conflict)
			}
		} else {
			return utils.LogError(log, ctx, "error check existing topics for namespace `%v': %w", ns, err)
		}
		return nil // destroy is permitted
	}

	if err := check(namespaces.Origin); err != nil {
		return err
	}
	return check(namespaces.Peer)

}

func (srv *KafkaServiceImpl) MigrateKafka(ctx context.Context) error {
	return srv.dao.Migrate(ctx)
}

func (srv *KafkaServiceImpl) copyVersionedTopics(ctx context.Context, fromNamespace, toNamespace string) error {
	trueVal := true
	topics, err := srv.SearchTopicsInDB(ctx, &model.TopicSearchRequest{
		Namespace: fromNamespace,
		Versioned: &trueVal,
	})
	if err != nil {
		return err
	}
	for _, topic := range topics {
		if topic.CreateReq == "" {
			return utils.LogError(log, ctx, "topic creation req is empty; need to apply topic configuration again: %w", msg.BadRequest)
		}
		var topicReqDto model.TopicRegistrationReqDto
		err := json.Unmarshal([]byte(topic.CreateReq), &topicReqDto)
		if err != nil {
			return utils.LogError(log, ctx, "can not unmarshal topic request DTO: :%s: %w", err.Error(), msg.BadRequest)
		}
		if topicReqDto.Name != "" &&
			(!strings.Contains(topicReqDto.Name, "{{namespace}}") && !strings.Contains(topicReqDto.Name, "%namespace%")) {
			return utils.LogError(log, ctx, "topic's name '%s' must contain {{namespace}} placeholder: %w", topicReqDto.Name, msg.BadRequest)
		}
		topic.Id = 0
		topic.Namespace = toNamespace
		topic.Classifier.Namespace = toNamespace
		topic.Topic = topicReqDto.Name

		_, _, err = srv.GetOrCreateTopicWithAuth(ctx, topic, model.Merge, func(ctx context.Context, namespace string, classifier *model.Classifier) error {
			return nil // ignore allowed namespaces check
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func wlog(ctx context.Context, m string, f func() error) error {
	log.InfoC(ctx, "Start %s", m)
	err := f()
	if err == nil {
		log.InfoC(ctx, "Finish %s", m)
	} else {
		log.ErrorC(ctx, "Error execute %s", m)
	}
	return err
}
