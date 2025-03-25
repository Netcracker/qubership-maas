package cr

import (
	"context"
	"maas/maas-service/model"
	"maas/maas-service/service/configurator_service"
	"maas/maas-service/service/rabbit_service"
)

type RabbitVhostConfigSpec struct {
	InstanceId string
	Entities   *model.RabbitEntities
	Policies   []interface{}
	Deletions  *model.RabbitEntities
	Classifier *RabbitVhostConfigSpecClassifier
}

type RabbitVhostConfigSpecClassifier struct {
	Name string
}

type RabbitHandler struct {
	configuratorService configurator_service.ConfiguratorService
	rabbitService       rabbit_service.RabbitService
}

func NewRabbitHandler(configuratorService configurator_service.ConfiguratorService, rabbitService rabbit_service.RabbitService) *RabbitHandler {
	return &RabbitHandler{configuratorService: configuratorService, rabbitService: rabbitService}
}

func (h RabbitHandler) HandleVhostConfiguration(ctx context.Context, config *RabbitVhostConfigSpec, metadata *CustomResourceMetadataRequest) (any, error) {
	return newHandler(ctx, config, metadata, &model.RabbitConfigReqDto{}).
		handle(nil, adaptVhostConfig, h.configuratorService.ApplyRabbitConfiguration)
}

func adaptVhostConfig(config *RabbitVhostConfigSpec, metadata *CustomResourceMetadataRequest) (*model.RabbitConfigReqDto, error) {
	var adaptedConfig model.RabbitConfigReqDto

	//no versioned entities anymore
	adaptedConfig.Spec.Entities = convertEntities(config.Entities)

	if config.Deletions != nil {
		adaptedConfig.Spec.RabbitDeletions = &model.RabbitDeletions{
			RabbitEntities: *convertEntities(config.Deletions),
		}
	}

	adaptedConfig.Spec.RabbitPolicies = config.Policies

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

	adaptedConfig.Spec.InstanceId = config.InstanceId

	return &adaptedConfig, nil
}

func convertEntities(entities *model.RabbitEntities) *model.RabbitEntities {
	cast := func(en []any) []any {
		result := make([]any, 0)
		for _, q := range en {
			items := make(map[string]any)
			for k, v := range q.(CustomResourceSpecRequest) {
				items[k] = v
			}
			result = append(result, items)
		}
		return result
	}

	return &model.RabbitEntities{
		Exchanges: cast(entities.Exchanges),
		Queues:    cast(entities.Queues),
		Bindings:  cast(entities.Bindings),
	}
}
