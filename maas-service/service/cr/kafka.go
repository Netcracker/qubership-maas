package cr

import (
	"context"
	"errors"
	"github.com/mitchellh/mapstructure"
	"maas/maas-service/model"
	"maas/maas-service/service/configurator_service"
	"maas/maas-service/service/kafka"
	"maas/maas-service/utils"
	"reflect"
)

var ErrDependencyNotFound = errors.New("required dependency is not found")

type preChecker[C any] func(ctx context.Context, config *C, metadata *CustomResourceMetadataRequest) error
type argAdapter[C, R any] func(config *C, metadata *CustomResourceMetadataRequest) (*R, error)
type executor func(ctx context.Context, cfg any, namespace string) (any, error)

type KafkaTopicConfig struct {
	TopicNameTemplate *string
	Pragma            map[string]any
	ExternallyManaged *bool
	Instance          string
	NumPartitions     *int
	MinNumPartitions  *int
	ReplicationFactor interface{}
	ReplicaAssignment map[int32][]int32
	Configs           map[string]*string
	Template          *string
	Versioned         bool
	Classifier        *KafkaTopicConfigClassifier
}

type KafkaTopicConfigClassifier struct {
	Name string
}

type KafkaTopicTemplateConfig struct {
	Pragma            map[string]any
	NumPartitions     *int
	MinNumPartitions  *int
	ReplicationFactor interface{}
	ReplicaAssignment map[int32][]int32
	Configs           map[string]string
}

type KafkaHandler struct {
	configuratorService configurator_service.ConfiguratorService
	kafkaService        kafka.KafkaService
}

func NewKafkaHandler(configuratorService configurator_service.ConfiguratorService, kafkaService kafka.KafkaService) *KafkaHandler {
	return &KafkaHandler{configuratorService: configuratorService, kafkaService: kafkaService}
}

func (h KafkaHandler) HandleTopicConfiguration(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicRegistrationConfigReqDto{}).
		handle(h.checkTopicTemplate, adaptTopicConfig, h.configuratorService.ApplyKafkaConfiguration)
}

func (h KafkaHandler) HandleLazyTopicConfiguration(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicRegistrationConfigReqDto{}).
		handle(h.checkTopicTemplate, adaptTopicConfig, h.configuratorService.ApplyKafkaLazyTopic)
}

func (h KafkaHandler) HandleTenantTopicConfiguration(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicRegistrationConfigReqDto{}).
		handle(h.checkTopicTemplate, adaptTopicConfig, h.configuratorService.ApplyKafkaTenantTopic)
}

func (h KafkaHandler) HandleTopicTemplateConfiguration(ctx context.Context, config *KafkaTopicTemplateConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicTemplateConfigReqDto{}).
		handle(nil, adaptTopicTemplateConfig, h.configuratorService.ApplyKafkaTopicTemplate)
}

func (h KafkaHandler) HandleDeleteTopicConfiguration(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicDeleteConfig{}).
		handle(nil, adaptDeleteTopicConfig, h.configuratorService.ApplyKafkaDeleteTopic)
}

func (h KafkaHandler) HandleDeleteTenantTopicConfiguration(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicDeleteConfig{}).
		handle(nil, adaptDeleteTopicConfig, h.configuratorService.ApplyKafkaDeleteTenantTopic)
}

func (h KafkaHandler) HandleDeleteTopicTemplateConfiguration(ctx context.Context, config *KafkaTopicTemplateConfig, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.TopicTemplateDeleteConfig{}).
		handle(nil, adaptDeleteTopicTemplateConfig, h.configuratorService.ApplyKafkaDeleteTopicTemplate)
}

func (h KafkaHandler) checkTopicTemplate(ctx context.Context, config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) error {
	if config.Template == nil {
		return nil
	}
	template, err := h.kafkaService.GetTopicTemplateByNameAndNamespace(ctx, *config.Template, metadata.Namespace)
	if err != nil {
		return err
	}
	if template == nil {
		log.WarnC(ctx, "required template with name '%s' is not found", *config.Template)
		return utils.LogError(log, ctx, "required template with name '%s' is not found: %w", *config.Template, ErrDependencyNotFound)
	}
	return nil
}

type handler[C, R any] struct {
	ctx      context.Context
	config   *C
	metadata *CustomResourceMetadataRequest
}

func newHandler[C, R any](ctx context.Context, config *C, metadata *CustomResourceMetadataRequest, _ *R) *handler[C, R] {
	return &handler[C, R]{
		ctx:      ctx,
		config:   config,
		metadata: metadata,
	}
}

func (h handler[C, R]) handle(preCheck preChecker[C], argAdapt argAdapter[C, R], exec executor) (any, error) {
	if preCheck != nil {
		if err := preCheck(h.ctx, h.config, h.metadata); err != nil {
			return nil, err
		}
	}

	adaptedConfig, err := argAdapt(h.config, h.metadata)
	if err != nil {
		return nil, err
	}
	configuration, err := exec(h.ctx, adaptedConfig, h.metadata.Namespace)
	return configuration, err
}

func adaptTopicConfig(config *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (*model.TopicRegistrationConfigReqDto, error) {
	var adaptedConfig model.TopicRegistrationConfigReqDto
	specDecoder, err := newDecoder(&adaptedConfig.Spec)
	if err != nil {
		return nil, err
	}
	err = specDecoder.Decode(config)
	if err != nil {
		return nil, err
	}

	pragmaDecoder, err := newDecoder(&adaptedConfig.Pragma)
	if err != nil {
		return nil, err
	}
	err = pragmaDecoder.Decode(config.Pragma)
	if err != nil {
		return nil, err
	}

	classifierDecoder, err := newDecoder(&adaptedConfig.Spec.Classifier)
	if err != nil {
		return nil, err
	}
	err = classifierDecoder.Decode(metadata)
	if err != nil {
		return nil, err
	}
	if config.Classifier != nil {
		err = classifierDecoder.Decode(config.Classifier)
		if err != nil {
			return nil, err
		}
	}

	return &adaptedConfig, nil
}

func adaptTopicTemplateConfig(config *KafkaTopicTemplateConfig, metadata *CustomResourceMetadataRequest) (*model.TopicTemplateConfigReqDto, error) {
	var adaptedConfig model.TopicTemplateConfigReqDto
	specDecoder, err := newDecoder(&adaptedConfig.Spec)
	if err != nil {
		return nil, err
	}
	err = specDecoder.Decode(config)
	if err != nil {
		return nil, err
	}

	pragmaDecoder, err := newDecoder(&adaptedConfig.Pragma)
	if err != nil {
		return nil, err
	}
	err = pragmaDecoder.Decode(config.Pragma)
	if err != nil {
		return nil, err
	}
	adaptedConfig.Spec.Name = metadata.Name
	return &adaptedConfig, nil
}

func adaptDeleteTopicTemplateConfig(_ *KafkaTopicTemplateConfig, metadata *CustomResourceMetadataRequest) (*model.TopicTemplateDeleteConfig, error) {
	var adaptedConfig model.TopicTemplateDeleteConfig
	adaptedConfig.Spec = &model.TopicTemplateDeleteCriteria{}
	specDecoder, err := newDecoder(&adaptedConfig.Spec)
	if err != nil {
		return nil, err
	}
	err = specDecoder.Decode(metadata)
	if err != nil {
		return nil, err
	}
	return &adaptedConfig, nil
}

func adaptDeleteTopicConfig(_ *KafkaTopicConfig, metadata *CustomResourceMetadataRequest) (*model.TopicDeleteConfig, error) {
	var adaptedConfig model.TopicDeleteConfig
	adaptedConfig.Spec = &model.TopicDeleteCriteria{}
	specDecoder, err := newDecoder(&adaptedConfig.Spec.Classifier)
	if err != nil {
		return nil, err
	}
	err = specDecoder.Decode(metadata)
	if err != nil {
		return nil, err
	}
	return &adaptedConfig, nil
}

func newDecoder(res any, hookFunc ...mapstructure.DecodeHookFunc) (*mapstructure.Decoder, error) {
	return mapstructure.NewDecoder(&mapstructure.DecoderConfig{
		DecodeHook: mapstructure.ComposeDecodeHookFunc(
			append(hookFunc, onEntityExistsParser())...,
		),
		Result: res,
	})
}

func onEntityExistsParser() func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
	return func(f reflect.Type, t reflect.Type, data interface{}) (interface{}, error) {
		if f.Kind() != reflect.String {
			return data, nil
		}
		var onEntityExistsEnum model.OnEntityExistsEnum
		if t == reflect.TypeOf(onEntityExistsEnum) {
			if data.(string) == "merge" {
				return model.Merge, nil
			}
			return model.Fail, nil
		}
		return data, nil
	}
}
