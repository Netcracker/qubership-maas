package cr

import (
	"context"

	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
)

type RabbitVhostConfigSpec struct {
	InstanceId string
	Entities   *model.RabbitEntities
	Policies   []interface{}
	Deletions  *model.RabbitDeletions
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
	if config.Entities != nil {
		adaptedConfig.Spec.Entities = convertEntities(config.Entities)
	}

	if config.Deletions != nil {
		adaptedConfig.Spec.RabbitDeletions = &model.RabbitDeletions{
			RabbitEntities: *convertEntities(&config.Deletions.RabbitEntities),
			RabbitPolicies: *convertPolicies(&config.Deletions.RabbitPolicies),
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
	return &model.RabbitEntities{
		Exchanges: toSpecMapSlice(entities.Exchanges),
		Queues:    toSpecMapSlice(entities.Queues),
		Bindings:  toSpecMapSlice(entities.Bindings),
	}
}

func convertPolicies(policies *model.RabbitPolicies) *model.RabbitPolicies {
	return &model.RabbitPolicies{
		Policies: toSpecMapSlice(policies.Policies),
	}
}

// toSpecMapSlice converts a slice of CustomResourceSpecRequest (as []interface{})
// into a slice of map[string]any to be sent further downstream.
func toSpecMapSlice(en []interface{}) []any {
	result := make([]any, 0, len(en))
	for _, q := range en {
		items := make(map[string]any)
		for k, v := range q.(CustomResourceSpecRequest) {
			items[k] = v
		}
		result = append(result, items)
	}
	return result
}
