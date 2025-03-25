package configurator_service

import (
	"context"
	"maas/maas-service/service/bg_service"
	"maas/maas-service/service/instance"
	"maas/maas-service/service/kafka"
	"maas/maas-service/service/rabbit_service"
)

type ConfiguratorServiceV2 struct {
	DefaultConfiguratorService
}

func NewConfiguratorServiceV2(
	kafkaInstanceService instance.KafkaInstanceService,
	rabbitInstanceService instance.RabbitInstanceService,
	rabbitService rabbit_service.RabbitService,
	tenantService TenantService,
	kafkaService kafka.KafkaService,
	bgService bg_service.BgService) *ConfiguratorServiceV2 {

	return &ConfiguratorServiceV2{DefaultConfiguratorService{
		kafkaInstanceService:  kafkaInstanceService,
		rabbitInstanceService: rabbitInstanceService,
		rabbitService:         rabbitService,
		tenantService:         tenantService,
		kafkaService:          kafkaService,
		bgService:             bgService,
	},
	}
}

func (cs *ConfiguratorServiceV2) ApplyKafkaConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaConfiguration(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaTopicTemplate(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaLazyTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaLazyTopic(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaTenantTopic(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaDeleteTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaDeleteTopic(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaDeleteTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaDeleteTenantTopic(ctx, cfg, namespace)
}

func (cs *ConfiguratorServiceV2) ApplyKafkaDeleteTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return cs.DefaultConfiguratorService.ApplyKafkaDeleteTopicTemplate(ctx, cfg, namespace)
}
