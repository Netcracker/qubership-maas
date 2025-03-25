package configurator_service

import (
	"context"
	"maas/maas-service/model"
	"maas/maas-service/service/bg_service"
	"maas/maas-service/service/instance"
	"maas/maas-service/service/kafka"
	"maas/maas-service/service/rabbit_service"
)

type ConfiguratorServiceV1 struct {
	DefaultConfiguratorService
}

func NewConfiguratorServiceV1(
	kafkaInstanceService instance.KafkaInstanceService,
	rabbitInstanceService instance.RabbitInstanceService,
	rabbitService rabbit_service.RabbitService,
	tenantService TenantService,
	kafkaService kafka.KafkaService,
	bgService bg_service.BgService) *ConfiguratorServiceV1 {

	return &ConfiguratorServiceV1{DefaultConfiguratorService{
		kafkaInstanceService:  kafkaInstanceService,
		rabbitInstanceService: rabbitInstanceService,
		rabbitService:         rabbitService,
		tenantService:         tenantService,
		kafkaService:          kafkaService,
		bgService:             bgService,
	}}
}

func (cs *ConfiguratorServiceV1) ApplyKafkaConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	result, err := cs.DefaultConfiguratorService.ApplyKafkaConfiguration(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}
	return toV1(result), nil
}

func (cs *ConfiguratorServiceV1) ApplyRabbitConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	result, err := cs.DefaultConfiguratorService.ApplyRabbitConfiguration(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}
	return toV1(result), nil
}

func (cs *ConfiguratorServiceV1) ApplyKafkaTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	result, err := cs.DefaultConfiguratorService.ApplyKafkaTopicTemplate(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}
	return toV1(result), nil
}

func (cs *ConfiguratorServiceV1) ApplyKafkaLazyTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	result, err := cs.DefaultConfiguratorService.ApplyKafkaLazyTopic(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}
	return toV1(result), nil
}

func (cs *ConfiguratorServiceV1) ApplyKafkaTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	result, err := cs.DefaultConfiguratorService.ApplyKafkaTenantTopic(ctx, cfg, namespace)
	if err != nil {
		return nil, err
	}
	return toV1(result), nil
}

func toV1(dto interface{}) interface{} {
	if c, ok := dto.(*model.TopicRegistrationRespDto); ok && c.RequestedSettings != nil {
		c.RequestedSettings.MinNumPartitions = nil
		return c
	}
	return dto
}
