package configurator_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	v "github.com/go-playground/validator/v10"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/bg_service"
	"github.com/netcracker/qubership-maas/service/composite"
	"github.com/netcracker/qubership-maas/service/instance"
	"github.com/netcracker/qubership-maas/service/kafka"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/validator"
	"golang.org/x/exp/slices"
	"math"
	"reflect"
	"sort"
	"strings"
)

const (
	RabbitConfigV1ApiVersion   = "nc.maas.rabbit/v1"
	RabbitConfigV2ApiVersion   = "nc.maas.rabbit/v2"
	KafkaConfigV1ApiVersion    = "nc.maas.kafka/v1"
	KafkaConfigV2ApiVersion    = "nc.maas.kafka/v2"
	AggregatedConfigApiVersion = "nc.maas.config/v2"
	CoreNcV1ApiVersion         = "core.netcracker.com/v1"
)

const (
	AggregatedConfigKind         = "config"
	RabbitConfigKind             = "vhost"
	TopicConfigKind              = "topic"
	InstanceDesignatorConfigKind = "instance-designator"
	TopicTemplateConfigKind      = "topic-template"
	LazyTopicConfigKind          = "lazy-topic"
	TenantTopicConfigKind        = "tenant-topic"
	TopicDeleteKind              = "topic-delete"
	TenantTopicDeleteKind        = "tenant-topic-delete"
	TopicTemplateDeleteKind      = "topic-template-delete"
)

const (
	CustomResourceKindTopic         = "Topic"
	CustomResourceKindTopicTemplate = "TopicTemplate"
	CustomResourceKindLazyTopic     = "LazyTopic"
	CustomResourceKindTenantTopic   = "TenantTopic"

	CustomResourceKindVhost = "VHost"
)

const (
	NoOrder            = math.MaxInt
	FirstOrder         = math.MinInt
	TopicTemplateOrder = FirstOrder + 1
)
const SHARED_CONFIG_SERVICE_NAME = ""

var log logging.Logger

func init() {
	log = logging.GetLogger("configurator_service")
}

type KindProcessor struct {
	order   int
	value   interface{}
	handler func(context.Context, interface{}, string) (interface{}, error)
}

type RegistrationService interface {
	Upsert(ctx context.Context, registrationRequest *composite.CompositeRegistration) error
	GetByNamespace(ctx context.Context, baseline string) (*composite.CompositeRegistration, error)
}

type DefaultConfiguratorService struct {
	registry              *registry
	kafkaInstanceService  instance.KafkaInstanceService
	rabbitInstanceService instance.RabbitInstanceService
	rabbitService         rabbit_service.RabbitService
	kafkaService          kafka.KafkaService
	bgService             bg_service.BgService
	bgDomain              domain.BGDomainService
	registrationService   RegistrationService
	tenantService         TenantService
}

//go:generate mockgen -source=configurator_service.go -destination=mock/configurator_service.go --package mock
type TenantService interface {
	GetTenantsByNamespace(ctx context.Context, namespace string) ([]model.Tenant, error)
}

type ConfiguratorService interface {
	ApplyConfig(ctx context.Context, raw string, namespace string) ([]model.ConfigMsResponse, error)
	ApplyConfigV2(ctx context.Context, raw string) ([]model.ConfigMsResponse, error)
	ApplyKafkaConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
	ApplyKafkaTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
	ApplyKafkaLazyTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
	ApplyKafkaTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
	ApplyKafkaDeleteTopic(ctx context.Context, cfg interface{}, _ string) (interface{}, error)
	ApplyKafkaDeleteTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
	ApplyKafkaDeleteTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)

	ApplyRabbitConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error)
}

func NewConfiguratorService(
	kafkaInstanceService instance.KafkaInstanceService,
	rabbitInstanceService instance.RabbitInstanceService,
	rabbitService rabbit_service.RabbitService,
	tenantService TenantService,
	kafkaService kafka.KafkaService,
	bgService bg_service.BgService,
	bgDomain domain.BGDomainService,
	registrationService RegistrationService) *DefaultConfiguratorService {

	cs := DefaultConfiguratorService{
		kafkaInstanceService:  kafkaInstanceService,
		rabbitInstanceService: rabbitInstanceService,
		rabbitService:         rabbitService,
		tenantService:         tenantService,
		kafkaService:          kafkaService,
		bgService:             bgService,
		bgDomain:              bgDomain,
		registrationService:   registrationService,
	} // [(ms, conf), ... ]
	csv1 := NewConfiguratorServiceV1(kafkaInstanceService, rabbitInstanceService, rabbitService, tenantService, kafkaService, bgService)
	csv2 := NewConfiguratorServiceV2(kafkaInstanceService, rabbitInstanceService, rabbitService, tenantService, kafkaService, bgService)

	cs.registry = newRegistry()
	cs.registry.add(FirstOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicDeleteKind}, &KindProcessor{value: model.TopicDeleteConfig{}, handler: csv2.ApplyKafkaDeleteTopic})
	cs.registry.add(FirstOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TenantTopicDeleteKind}, &KindProcessor{value: model.TopicDeleteConfig{}, handler: csv2.ApplyKafkaDeleteTenantTopic})
	cs.registry.add(FirstOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicTemplateDeleteKind}, &KindProcessor{value: model.TopicTemplateDeleteConfig{}, handler: csv2.ApplyKafkaDeleteTopicTemplate})

	cs.registry.add(FirstOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: InstanceDesignatorConfigKind}, &KindProcessor{value: model.InstanceDesignatorKafkaReq{}, handler: cs.applyInstanceDesignatorKafka})
	cs.registry.add(FirstOrder, model.Kind{ApiVersion: RabbitConfigV2ApiVersion, Kind: InstanceDesignatorConfigKind}, &KindProcessor{value: model.InstanceDesignatorRabbitReq{}, handler: cs.applyInstanceDesignatorRabbit})

	cs.registry.add(TopicTemplateOrder, model.Kind{ApiVersion: KafkaConfigV1ApiVersion, Kind: TopicTemplateConfigKind}, &KindProcessor{value: model.TopicTemplateConfigReqDto{}, handler: csv1.ApplyKafkaTopicTemplate})
	cs.registry.add(TopicTemplateOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicTemplateConfigKind}, &KindProcessor{value: model.TopicTemplateConfigReqDto{}, handler: csv2.ApplyKafkaTopicTemplate})

	cs.registry.add(NoOrder, model.Kind{ApiVersion: RabbitConfigV1ApiVersion, Kind: RabbitConfigKind}, &KindProcessor{value: model.RabbitConfigReqDto{}, handler: cs.ApplyRabbitConfiguration})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: RabbitConfigV2ApiVersion, Kind: RabbitConfigKind}, &KindProcessor{value: model.RabbitConfigReqDto{}, handler: cs.applyRabbitConfigurationV2})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV1ApiVersion, Kind: TopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv1.ApplyKafkaConfiguration})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv2.ApplyKafkaConfiguration})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV1ApiVersion, Kind: LazyTopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv1.ApplyKafkaLazyTopic})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: LazyTopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv2.ApplyKafkaLazyTopic})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV1ApiVersion, Kind: TenantTopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv1.ApplyKafkaTenantTopic})
	cs.registry.add(NoOrder, model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TenantTopicConfigKind}, &KindProcessor{value: model.TopicRegistrationConfigReqDto{}, handler: csv2.ApplyKafkaTenantTopic})

	return &cs
}

type registry struct {
	order   map[model.Kind]int
	content map[model.Kind]*KindProcessor
}

func newRegistry() *registry {
	return &registry{
		order:   make(map[model.Kind]int),
		content: make(map[model.Kind]*KindProcessor),
	}
}
func (r registry) add(order int, kind model.Kind, processor *KindProcessor) {
	r.order[kind] = order
	r.content[kind] = processor
}
func (r registry) getProcessor(kind model.Kind) *KindProcessor {
	return r.content[kind]
}

func (r registry) orderedKinds() []model.Kind {
	result := make([]model.Kind, len(r.content))
	i := 0
	for kind := range r.content {
		result[i] = kind
		i++
	}
	sort.Slice(result, func(i, j int) bool {
		return r.order[result[i]] < r.order[result[j]]
	})

	return result
}

type AggregateConfigError struct {
	Err     error
	Message string
}

func (e AggregateConfigError) Error() string {
	return fmt.Sprintf("Error during applying aggregated config, error: '%s', message: (%s)", e.Err.Error(), e.Message)
}

func (cs *DefaultConfiguratorService) ApplyConfig(ctx context.Context, raw string, namespace string) ([]model.ConfigMsResponse, error) {
	normalized, err := utils.NormalizeJsonOrYamlInput(raw)
	if err != nil {
		return nil, err
	}

	var objects []interface{}
	if err = json.Unmarshal([]byte(normalized), &objects); err != nil {
		return nil, err
	}

	var results []model.ConfigMsResponse
	var firstError error = nil // save only first error for function return response
	appendErrorResp := func(obj interface{}, format string, v ...interface{}) {
		msg := fmt.Sprintf(format, v...)
		results = append(results,
			model.ConfigMsResponse{
				obj,
				model.ConfigMsResult{Status: "error", Error: msg},
			},
		)
		if firstError == nil {
			// save only first error
			firstError = errors.New(msg)
		}
	}

	for _, obj := range objects {
		// try to convert to Kind
		var kind model.Kind
		if err := utils.NarrowInputToOutput(obj, &kind); err != nil {
			log.ErrorC(ctx, "Invalid input structure given. Can't convert to ApiVersion/Kind type: %v", err)
			appendErrorResp(obj, "Invalid input structure given. Can't convert to ApiVersion/Kind type: %v", err)
			continue
		}
		if kind.ApiVersion == "" || kind.Kind == "" {
			log.ErrorC(ctx, "Empty ApiVersion/Kind values in: %v", obj)
			appendErrorResp(obj, "Empty ApiVersion/Kind values in: %v", obj)
			continue
		}

		if proc := cs.registry.getProcessor(kind); proc != nil {
			config := reflect.New(reflect.TypeOf(proc.value)).Interface()

			if err := utils.NarrowInputToOutputStrict(obj, config); err != nil {
				msg := fmt.Sprintf("can't map data in config v1 to: '%+v' for config: '%+v', for init object: '%+v'. error: %v", proc.value, config, obj, err)
				return results, errors.New(msg)
			}

			if result, err := proc.handler(ctx, config, namespace); err == nil {
				results = append(results, model.ConfigMsResponse{
					config,
					model.ConfigMsResult{Status: model.STATUS_OK, Data: result},
				})
			} else {
				log.ErrorC(ctx, "Error apply config: %v, \n\tConfig: %+v", err, config)
				var errMsg = err.Error()
				if ve, ok := err.(v.ValidationErrors); ok {
					errMsg = strings.Join(validator.FormatErrors(ve), "; ")
				}
				appendErrorResp(config, "Error apply config: %s, \n\tConfig: %+v", errMsg, config)
				continue
			}
		} else {
			log.ErrorC(ctx, "Unsupported object kind: %+v", kind)
			appendErrorResp(obj, "Unsupported object kind: %+v", kind)
			continue
		}
	}

	return results, firstError
}

func (cs *DefaultConfiguratorService) ApplyConfigV2(ctx context.Context, raw string) ([]model.ConfigMsResponse, error) {
	log.InfoC(ctx, "Starting to parse YAML config")
	namespace := model.RequestContextOf(ctx).Namespace

	aggregatedConfig, err := parseAggregatedConfigYAML(raw)
	if err != nil {
		log.ErrorC(ctx, "Error during parseAggregatedConfigYAML while in ApplyConfigV2: %v", err)
		return nil, err
	}

	if aggregatedConfig.Spec.BaseNamespace != "" {
		if err = cs.updateCompositeRegistration(ctx, namespace, aggregatedConfig.Spec.BaseNamespace); err != nil {
			return nil, err
		}
	}

	//from now on we cycle through first shared then ms configs to put them to kindToServiceConfigs map,
	//in case of error we DON'T need to return successful results of previous configs, because they are not processed yet,
	//we can just fail whole config as bad input

	//map of service configs per different types to process them later
	kindToServiceConfigs := map[model.Kind][]model.ServiceConfig{}

	//parsing shared configs
	if aggregatedConfig.Spec.SharedConfigs != nil {
		if sharedConfig, ok := aggregatedConfig.Spec.SharedConfigs.(string); ok {
			//shared config has empty service name
			if err := cs.parseInnerConfigsOfMs(kindToServiceConfigs, sharedConfig, SHARED_CONFIG_SERVICE_NAME); err != nil {
				return nil, utils.LogError(log, ctx, "Error during parseInnerConfigsOfMs for shared config while in ApplyConfigV2: %w", err)
			}
		} else {
			return nil, utils.LogError(log, ctx, "Config parse error: `spec.shared' field should contain plain string value")
		}
	}

	//we need to store empty configs, because it could be update case for rabbit, rabbit ms could have had config before, now it is empty and meant to delete entities
	var msNamesWithEmptyConfig []string
	for _, unparsedServiceConfig := range aggregatedConfig.Spec.ServiceConfigs {
		//we need to store empty configs, because it could be update case, rabbit ms could have had config before, now it is empty and meant to delete entities
		if unparsedServiceConfig.Config == nil || unparsedServiceConfig.Config == "" {
			msNamesWithEmptyConfig = append(msNamesWithEmptyConfig, unparsedServiceConfig.ServiceName)
			continue
		}

		err := cs.parseInnerConfigsOfMs(kindToServiceConfigs, unparsedServiceConfig.Config.(string), unparsedServiceConfig.ServiceName)
		if err != nil {
			log.ErrorC(ctx, "Error during parseInnerConfigsOfMs for shared config while in ApplyConfigV2: %v", err)
			return nil, err
		}
	}
	log.InfoC(ctx, "Whole YAML config was successfully parsed")

	//if no config with rabbit - we created msNamesWithEmptyConfig to put empty configs in case of update, if user decided to clear all entities
	//if no config at all - we should have only structures like {serviceName: ***, config: }

	//adding empty configs for rabbit
	for _, msName := range msNamesWithEmptyConfig {
		kindToServiceConfigs[model.Kind{ApiVersion: RabbitConfigV2ApiVersion, Kind: RabbitConfigKind}] = append(kindToServiceConfigs[model.Kind{ApiVersion: RabbitConfigV2ApiVersion, Kind: RabbitConfigKind}], model.ServiceConfig{
			ServiceName: msName,
			Config:      nil,
		})
	}

	var aggregateResult []model.ConfigMsResponse

	//we save first error to return it on outer level of response, inner errors are described for any specific ms inside it anyway
	//so if no inner errors, but outer - user will see outer overall error (as for rabbit validation)
	//if there is inner error, then user will see first inner error for kafka and we continue to process other configs if mistake is in one of kafka's configs
	//or if it is rabbit error, then user will see first inner error for rabbit and no more rabbit configs are processed
	var firstError error = nil

	for _, kind := range cs.registry.orderedKinds() {
		serviceConfigs, found := kindToServiceConfigs[kind]
		if !found {
			continue
		}
		log.InfoC(ctx, "Processing service configs of kind: %v", kind)
		kindProcessor := cs.registry.getProcessor(kind)

		//we process rabbit in specific way - all configs are handled together, unlike other kinds
		if kind.ApiVersion == RabbitConfigV2ApiVersion && kind.Kind == RabbitConfigKind {
			//distributed transactions on whole namespace (config is applied for all vhosts of namespace) to prevent desync of Rabbit and DB
			rabbitResultsMap, err := dao.WithTransactionContextValue(ctx, func(ctx context.Context) (map[string]model.RabbitResult, error) {
				//send whole array
				msRabbitResults, err := kindProcessor.handler(ctx, model.ServiceConfigs{
					Version:        aggregatedConfig.Spec.Version,
					Namespace:      aggregatedConfig.Spec.Namespace,
					ServiceConfigs: serviceConfigs,
				}, namespace)

				return msRabbitResults.(map[string]model.RabbitResult), err
			})
			if err != nil {
				log.ErrorC(ctx, "error during applying aggregated rabbit config: %w", err)
			}

			for _, serviceConfig := range serviceConfigs {
				innerErr := rabbitResultsMap[serviceConfig.ServiceName].Error
				if innerErr == nil {
					aggregateResult = append(aggregateResult, model.ConfigMsResponse{
						Request: serviceConfig,
						Result:  model.ConfigMsResult{Status: model.STATUS_OK, Data: rabbitResultsMap[serviceConfig.ServiceName]},
					})
				} else {
					if _, ok := innerErr.(model.AggregateConfigError); !ok {
						innerErr = model.AggregateConfigError{
							Err:     model.ErrAggregateConfigInternalMsError,
							Message: fmt.Sprintf("Error during applying rabbit inner config for microservice with name '%v', err: %v", serviceConfig.ServiceName, innerErr.Error()),
						}
					}
					log.ErrorC(ctx, "Error during applying rabbit inner config: %v", innerErr)
					appendErrorResp(serviceConfig, innerErr, &aggregateResult, &firstError)
				}
			}

			//checking overall error if there were no inner errors
			if firstError == nil && err != nil {
				//check if error has been already typed
				if _, ok := err.(model.AggregateConfigError); !ok {
					err = model.AggregateConfigError{
						Err:     model.ErrAggregateConfigOverallRabbitError,
						Message: fmt.Sprintf("err: %v", err.Error()),
					}
				}
				//then error is general, not in specific config
				firstError = err
			}
		} else {
			//for all kinds except rabbit v2 configs are handled separately, their responses are appended inside configApplicationResponse
			for _, serviceConfig := range serviceConfigs {
				if result, err := kindProcessor.handler(ctx, serviceConfig.Config, namespace); err == nil {
					aggregateResult = append(aggregateResult, model.ConfigMsResponse{
						Request: serviceConfig,
						Result:  model.ConfigMsResult{Status: model.STATUS_OK, Data: result},
					})
				} else {
					if _, ok := err.(model.AggregateConfigError); !ok {
						err = model.AggregateConfigError{
							Err:     model.ErrAggregateConfigInternalMsError,
							Message: fmt.Sprintf("Error during applying inner config for microservice with name '%v', err: %v", serviceConfig.ServiceName, err.Error()),
						}
					}
					log.ErrorC(ctx, "Error during applying inner config: %v", err)
					appendErrorResp(serviceConfig, err, &aggregateResult, &firstError)
				}
			}
		}
		log.InfoC(ctx, "End of processing service configs with kind: %v", kind)
	}
	return aggregateResult, firstError
}

func parseAggregatedConfigYAML(rawConfig string) (*model.ConfigReqDto, error) {
	normalizedReq, err := utils.NormalizeJsonOrYamlInput(rawConfig)
	if err != nil {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("Can't parse input YAML: %v", err),
		}
	}

	//in current implementation (Rabbit BG feature) single aggregated config comes from deployer, but we support it as array
	var initYamlsSlices []interface{}
	if err = json.Unmarshal([]byte(normalizedReq), &initYamlsSlices); err != nil {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("Can't unmarshal input YAML: %v", err),
		}
	}
	if len(initYamlsSlices) != 1 {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: "Only single aggregated YAML file is supported",
		}
	}
	initYaml := initYamlsSlices[0]

	var kind model.Kind
	if err := utils.NarrowInputToOutput(initYaml, &kind); err != nil {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("Invalid input structure given. Can't convert to ApiVersion/Kind type: %v", err),
		}
	}
	if kind.ApiVersion != AggregatedConfigApiVersion || kind.Kind != AggregatedConfigKind {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("Not supported apiVersion or kind in YAML, it should be apiVersion == '%v', kind == '%v'", AggregatedConfigApiVersion, AggregatedConfigKind),
		}
	}

	aggregatedConfig := model.ConfigReqDto{}
	if err := utils.NarrowInputToOutputStrict(initYaml, &aggregatedConfig); err != nil {
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("can't map data in aggregated config to: %+v, for init yaml: '%+v'. error: %v", model.ConfigReqDto{}, initYaml, err),
		}
	}

	return &aggregatedConfig, nil
}

// golang passes map by reference
func (cs *DefaultConfiguratorService) parseInnerConfigsOfMs(kindToServiceConfigs map[model.Kind][]model.ServiceConfig, msConfig string, msName string) error {
	normalizedMsConfig, err := utils.NormalizeJsonOrYamlInput(msConfig)
	if err != nil {
		return model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("can't NormalizeJsonOrYamlInput during parseInnerConfigsOfMs for '%v'. error: %v", msConfig, err),
		}
	}

	//ms config can have several inner configs of different kinds inside
	var innerConfigs []interface{}
	if err = json.Unmarshal([]byte(normalizedMsConfig), &innerConfigs); err != nil {
		return model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("can't unmarshal innerConfigsg during parseInnerConfigsOfMs for '%v', error: %v", msConfig, err),
		}
	}

	for _, innerConfig := range innerConfigs {
		var kind model.Kind
		if err := utils.NarrowInputToOutput(innerConfig, &kind); err != nil {
			return model.AggregateConfigError{
				Err:     model.ErrAggregateConfigParsing,
				Message: fmt.Sprintf("Invalid input structure given. Can't convert inner config '%v' to ApiVersion/Kind type, error: %v", innerConfig, err),
			}
		}
		if kind.ApiVersion == "" || kind.Kind == "" {
			return model.AggregateConfigError{
				Err:     model.ErrAggregateConfigParsing,
				Message: fmt.Sprintf("Invalid input structure given. Empty ApiVersion/Kind values in inner config '%v'", innerConfig),
			}
		}

		if kindProcessor := cs.registry.getProcessor(kind); kindProcessor == nil {
			return AggregateConfigError{
				Err:     model.ErrAggregateConfigParsing,
				Message: fmt.Sprintf("Unsupported object kind '%+v' in inner config '%v'", kind, innerConfig),
			}
		} else {
			typedInnerConfig := reflect.New(reflect.TypeOf(kindProcessor.value)).Interface()
			if err := utils.NarrowInputToOutputStrict(innerConfig, typedInnerConfig); err != nil {
				return model.AggregateConfigError{
					Err:     model.ErrAggregateConfigParsing,
					Message: fmt.Sprintf("can't map data in config to '%+v', for config '%v'. error: '%v'", kindProcessor.value, innerConfig, err),
				}
			}

			kindToServiceConfigs[kind] = append(kindToServiceConfigs[kind], model.ServiceConfig{ServiceName: msName, Config: typedInnerConfig})
		}
	}
	return nil
}

func appendErrorResp(config interface{}, err error, aggregateResult *[]model.ConfigMsResponse, firstError *error) {
	//skip if no error
	if err == nil {
		return
	}

	*aggregateResult = append(*aggregateResult, model.ConfigMsResponse{
		Request: config,
		Result:  model.ConfigMsResult{Status: model.STATUS_ERROR, Error: err.Error()},
	},
	)

	//apply only first error to outer response
	if *firstError == nil {
		*firstError = err
	}
}

func (cs *DefaultConfiguratorService) applyInstanceDesignatorKafka(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	config, ok := cfg.(*model.InstanceDesignatorKafkaReq)
	if !ok {
		msg := fmt.Sprintf("Problem during casting cfg to InstanceDesignatorKafkaReq for config: %v", cfg)
		log.ErrorC(ctx, msg)
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigConversionError,
			Message: msg,
		}
	}
	instanceDesignator := config.Spec

	err := cs.kafkaInstanceService.UpsertKafkaInstanceDesignator(ctx, instanceDesignator, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during applying instance designator kafka: %v", err)
		return nil, err
	}

	log.InfoC(ctx, "Applying instance designator kafka was successful for namespace '%v'", namespace)
	return nil, nil
}

func (cs *DefaultConfiguratorService) applyInstanceDesignatorRabbit(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	config, ok := cfg.(*model.InstanceDesignatorRabbitReq)
	if !ok {
		msg := fmt.Sprintf("Problem during casting cfg to InstanceDesignatorRabbitReq for config: %v", cfg)
		log.ErrorC(ctx, msg)
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigConversionError,
			Message: msg,
		}
	}

	instanceDesignator := config.Spec

	err := cs.rabbitInstanceService.UpsertRabbitInstanceDesignator(ctx, instanceDesignator, namespace)
	if err != nil {
		log.ErrorC(ctx, "Error during applying instance designator rabbit: %v", err)
		return nil, err
	}

	log.InfoC(ctx, "Applying instance designator rabbit was successful for namespace '%v'", namespace)
	return nil, nil
}

func (cs *DefaultConfiguratorService) ApplyRabbitConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	config := cfg.(*model.RabbitConfigReqDto)

	result := &model.RabbitResult{}

	if err := validator.Get().Struct(&config.Spec); err != nil {
		log.ErrorC(ctx, "Error in CheckFormat of classifier during ApplyRabbitConfiguration: %v", err)
		return nil, err
	}
	//TODO during migration we allow to use base or relative namespaces only for GET operations, later it should be with srv.dao.GetAllowedNamespaces(ctx, namespace)
	err := config.Spec.Classifier.CheckAllowedNamespaces(namespace)
	if err != nil {
		log.ErrorC(ctx, "Error in VerifyAuth of classifier during ApplyRabbitConfiguration: %v", err)
		return nil, err
	}

	//vhost section
	_, vHostRegistration, err := cs.rabbitService.GetOrCreateVhost(ctx, config.Spec.InstanceId, &config.Spec.Classifier, nil)
	if err != nil {
		log.ErrorC(ctx, "Error during vhost getting or creation: %v", err)
		return nil, err
	}
	result.VHostReg = *vHostRegistration

	cnnUrl, err := cs.rabbitService.GetConnectionUrl(ctx, vHostRegistration)
	if err != nil {
		log.ErrorC(ctx, "Error during GetConnectionUrl in ApplyRabbitConfiguration err: %v", err)
		return nil, err
	}

	result.VHostResp = model.VHostRegistrationResponse{
		Cnn:      cnnUrl,
		Username: vHostRegistration.User,
		Password: vHostRegistration.Password,
	}

	//deletion section
	if config.Spec.RabbitDeletions != nil {
		result.RabbitDeletions, err = cs.rabbitService.DeleteEntities(ctx, config.Spec.Classifier, *config.Spec.RabbitDeletions)
		if err != nil {
			log.ErrorC(ctx, "Error during deleting config: %v", err)
			return nil, err
		}
		log.InfoC(ctx, "RabbitResult deletions: %+v", result.RabbitDeletions)
	}

	if config.Spec.RabbitPolicies != nil {
		result.Policies, err = cs.rabbitService.ApplyPolicies(ctx, config.Spec.Classifier, config.Spec.RabbitPolicies)
		if err != nil {
			log.ErrorC(ctx, "Error during applying policies: %v", err)
			return nil, err
		}
		if result.Policies == nil {
			result.Policies = make([]interface{}, 0)
		}
		log.InfoC(ctx, "RabbitResult created policies: %+v", result.Policies)
	}

	//entities section
	if config.Spec.Entities != nil {
		result.Entities, result.UpdateStatus, err = cs.rabbitService.CreateOrUpdateEntitiesV1(ctx, vHostRegistration, *config.Spec.Entities)
		if err != nil {
			log.ErrorC(ctx, "Error during entities create or update: %v", err)
			return nil, err
		}
	}

	//exported vhost section
	err = cs.rabbitService.ProcessExportedVhost(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "Error during ProcessExportedVhost while in ApplyRabbitConfiguration: %v", err)
	}

	return result, nil
}

func (cs *DefaultConfiguratorService) applyRabbitConfigurationV2(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	var results map[string]model.RabbitResult
	var entitiesToBeDeleted []model.RabbitVersionedEntity

	log.InfoC(ctx, "Starting to apply rabbit configuration for namespace '%v'", namespace)
	serviceConfigs, ok := cfg.(model.ServiceConfigs)
	if !ok {
		errMsg := fmt.Sprintf("Error during converting config to 'ServiceConfigs', only aggregated config is allowed. Note, that you can't send rabbit config v2 to rabbit directly, it should be sent via deployed, which aggregates configs from all microservices of application")
		log.ErrorC(ctx, errMsg)
		return results, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: errMsg,
		}
	}

	//version set correctly previously, deployer doesn't know about bg2
	candidateVersion := serviceConfigs.Version

	log.InfoC(ctx, "Starting to apply rabbit config v1, add microservices' configs and their entities to DB")
	results, entitiesToBeDeleted, err := applyRabbitConfigV1AndServiceConfigsToDb(ctx, cs, serviceConfigs, namespace, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, err.Error())
		return results, err
	}
	log.InfoC(ctx, "All configs of microservices and their entities were successfully added to DB")

	activeVersion, err := cs.bgService.GetActiveVersionByNamespace(ctx, namespace)
	if err != nil {
		return results, utils.LogError(log, ctx, "error during GetActiveVersionByNamespace for bg1 for namespace '%v': %w", namespace, err)
	}

	// active version could be "" during the first deployment
	if activeVersion == "" {
		log.InfoC(ctx, "Current active version is not set, it will be set to v1 and default routes will be set to it")
		err = cs.bgService.ApplyBgStatus(ctx, namespace, &model.BgStatus{
			Active: model.INITIAL_VERSION,
		})
		if err != nil {
			log.ErrorC(ctx, "Error during ApplyBgStatus for default v1 version: %v", err)
			return results, err
		}
		activeVersion = model.INITIAL_VERSION
	}

	// ApplyMssInActiveButNotInCandidateForVhost part, only if it is not active version
	if activeVersion != candidateVersion {
		log.InfoC(ctx, "Current active version for namespace '%v' is '%v' != candidate version is '%v', starting to ApplyMssInActiveButNotInCandidateForVhost", namespace, activeVersion, candidateVersion)
		vhosts, err := cs.rabbitService.FindVhostsByNamespace(ctx, namespace)
		if err != nil {
			return results, utils.LogError(log, ctx, "error during FindVhostsByNamespace for namespace '%v': %w", namespace, err)
		}

		for _, vhost := range vhosts {
			err := cs.rabbitService.ApplyMssInActiveButNotInCandidateForVhost(ctx, vhost, activeVersion, candidateVersion)
			if err != nil {
				err = fmt.Errorf("error during ApplyMssInActiveButNotInCandidateForVhost for vhost with classifier '%v': %w", vhost.Classifier, err)
				log.ErrorC(ctx, err.Error())
				return results, err
			}
		}

	}

	//Validation with rules in comments, each is for vhost and version
	log.InfoC(ctx, "Starting validation of entities for namespace '%v'", namespace)
	err = cs.rabbitService.RabbitBgValidation(ctx, namespace, candidateVersion)
	if err != nil {
		log.ErrorC(ctx, err.Error())
		return results, err
	}

	//Delete entities from Rabbit
	log.InfoC(ctx, "Checking if there are unnecessary rabbit entities")
	_, err = cs.rabbitService.DeleteEntitiesByRabbitVersionedEntities(ctx, entitiesToBeDeleted)
	if err != nil {
		err = fmt.Errorf("error during DeleteEntitiesByRabbitVersionedEntities of entitiesToBeDeleted: %w", err)
		log.ErrorC(ctx, err.Error())
		return results, err
	}

	//CreateVersionedEntities in Rabbit per MsConfigsByNamespaceAndCandidateVersion
	log.InfoC(ctx, "Starting to create version entities in candidateVersion '%v'", candidateVersion)
	_, _, err = cs.rabbitService.CreateVersionedEntities(ctx, namespace, candidateVersion)
	if err != nil {
		err = fmt.Errorf("error during CreateVersionedEntities: %w", err)
		log.ErrorC(ctx, err.Error())
		return results, err
	}

	return results, nil
}

func applyRabbitConfigV1AndServiceConfigsToDb(ctx context.Context, cs *DefaultConfiguratorService, serviceConfigs model.ServiceConfigs, namespace string, candidateVersion string) (map[string]model.RabbitResult, []model.RabbitVersionedEntity, error) {
	results := make(map[string]model.RabbitResult)
	var entitiesToBeDeleted []model.RabbitVersionedEntity

	for _, serviceConfig := range serviceConfigs.ServiceConfigs {

		//we do not allow empty names from shared config for rabbit v2 config, bg is not possible if config is not linked to ms
		if serviceConfig.ServiceName == "" {
			err := fmt.Errorf("service name is empty for rabbit v2 config, bg is not possible if config is not linked to ms, you should not put rabbit v2 config to shared config")
			results[serviceConfig.ServiceName] = model.RabbitResult{Error: err}
			return results, entitiesToBeDeleted, err
		}

		//case when we got microservice with empty config - it could be signal to delete previous entities during update or not to create exchange and binding for a candidate
		//so we need to ApplyMsConfigAndVersionedEntitiesToDb with VersionedEntities == nil
		if serviceConfig.Config == nil {
			log.InfoC(ctx, "Rabbit configuration for microservice '%v' is nil, clearing previous versioned entities if they existed", serviceConfig.ServiceName)
			vhosts, err := cs.rabbitService.FindVhostsByNamespace(ctx, namespace)
			if err != nil {
				return results, entitiesToBeDeleted, utils.LogError(log, ctx, "error during FindVhostsByNamespace for namespace '%v': %w", namespace, err)
			}

			for _, vhost := range vhosts {
				rabbitConfig := &model.RabbitConfigReqDto{
					Kind:   model.Kind{},
					Pragma: nil,
					Spec: model.ApplyRabbitConfigReqDto{
						Classifier: func() model.Classifier {
							classifier, _ := model.ConvertToClassifier(vhost.Classifier)
							return classifier
						}(),
						InstanceId:        vhost.InstanceId,
						VersionedEntities: nil,
					},
				}

				deleted, err := cs.rabbitService.ApplyMsConfigAndVersionedEntitiesToDb(ctx, serviceConfig.ServiceName, rabbitConfig, vhost.Id, candidateVersion, namespace)
				if err != nil {
					if _, ok := err.(model.AggregateConfigError); !ok {
						err = fmt.Errorf("error during ApplyMsConfigAndVersionedEntitiesToDb: %w", err)
					}
					results[serviceConfig.ServiceName] = model.RabbitResult{Error: err}
					return results, entitiesToBeDeleted, err
				}
				entitiesToBeDeleted = append(entitiesToBeDeleted, deleted...)
			}
		} else {
			log.InfoC(ctx, "Rabbit configuration for microservice '%v' is not empty, applying non-versioned part", serviceConfig.ServiceName)
			//case when we got microservice with non-empty config - apply previous configuration v1 (non-versioned), then ApplyMsConfigAndVersionedEntitiesToDb
			rabbitConfig, ok := serviceConfig.Config.(*model.RabbitConfigReqDto)
			if !ok {
				err := fmt.Errorf("error during conversion to RabbitConfigReqDto for '%+v'", rabbitConfig)
				log.ErrorC(ctx, err.Error())
				return results, entitiesToBeDeleted, err
			}

			//vhost section
			_, vhostReg, err := cs.rabbitService.GetOrCreateVhost(ctx, rabbitConfig.Spec.InstanceId, &rabbitConfig.Spec.Classifier, nil)
			if err != nil {
				log.ErrorC(ctx, "Error during vhost getting or creation: %v", err)
				return results, entitiesToBeDeleted, err
			}

			//can't skip if no versioned entities, because they could exist in active and now should be deleted
			deleted, err := cs.rabbitService.ApplyMsConfigAndVersionedEntitiesToDb(ctx, serviceConfig.ServiceName, rabbitConfig, vhostReg.Id, candidateVersion, namespace)
			if err != nil {
				if _, ok := err.(model.AggregateConfigError); !ok {
					err = fmt.Errorf("error during ApplyMsConfigAndVersionedEntitiesToDb: %w", err)
				}
				results[serviceConfig.ServiceName] = model.RabbitResult{Error: err}
				return results, entitiesToBeDeleted, err
			}
			entitiesToBeDeleted = append(entitiesToBeDeleted, deleted...)

			interfaceResult, err := cs.ApplyRabbitConfiguration(ctx, rabbitConfig, namespace)
			if err != nil {
				err = fmt.Errorf("error during ApplyRabbitConfiguration: %w", err)
				results[serviceConfig.ServiceName] = model.RabbitResult{Error: err}
				return results, entitiesToBeDeleted, err
			}

			log.InfoC(ctx, "Rabbit configuration for microservice '%v' is not empty, applying versioned part", serviceConfig.ServiceName)
			result, ok := interfaceResult.(*model.RabbitResult)
			if !ok {
				err := fmt.Errorf("error during conversion to RabbitResult for '%+v'", interfaceResult)
				log.ErrorC(ctx, err.Error())
				return results, entitiesToBeDeleted, err
			}

			results[serviceConfig.ServiceName] = *result
		}
	}
	return results, entitiesToBeDeleted, nil
}

func (cs *DefaultConfiguratorService) ApplyKafkaConfiguration(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	config, ok := cfg.(*model.TopicRegistrationConfigReqDto)
	if !ok {
		err := fmt.Errorf("error during conversion to TopicRegistrationConfigReqDto for '%+v'", cfg)
		log.ErrorC(ctx, err.Error())
		return nil, err
	}
	log.DebugC(ctx, "apply config: %+v", config)

	onTopicExists := model.Fail
	if config.Pragma != nil {
		onTopicExists = config.Pragma.OnEntityExists
	}

	topicTemplate, err := cs.kafkaService.GetTopicTemplateByNameAndNamespace(ctx, config.Spec.Template, config.Spec.Classifier.Namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error resolve topic template: %w", err)
	}

	topic, err := config.Spec.BuildTopicRegFromReq(topicTemplate)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error resolve topic template settings: %w", err)
	}

	_, result, err := cs.kafkaService.GetOrCreateTopic(ctx, topic, onTopicExists)
	return result, err
}

func (cs *DefaultConfiguratorService) ApplyKafkaTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	config, ok := cfg.(*model.TopicTemplateConfigReqDto)
	if !ok {
		err := fmt.Errorf("error during conversion to TopicTemplateConfigReqDto for '%+v'", cfg)
		log.ErrorC(ctx, err.Error())
		return nil, err
	}
	log.DebugC(ctx, "Topic template: %+v", config)

	topicTemplateFromReq, err := config.Spec.BuildTopicTemplateFromReq(namespace)
	if err != nil {
		log.ErrorC(ctx, "bad request: %v, %v", config, err)
		return nil, err
	}

	log.InfoC(ctx, "Trying to get topic template by name %v and namespace %v", topicTemplateFromReq, namespace)
	topicTemplate, err := cs.kafkaService.GetTopicTemplateByNameAndNamespace(ctx, topicTemplateFromReq.Name, namespace)
	if err != nil {
		log.ErrorC(ctx, "error getting topic template with req: %v, %v", config, err)
		return nil, err
	}

	result := model.TopicTemplateRespDto{
		Name:      topicTemplateFromReq.Name,
		Namespace: topicTemplateFromReq.Namespace,
	}

	if topicTemplate == nil {
		log.InfoC(ctx, "No topic template with name %v and namespace %v was found. Creating...", topicTemplateFromReq, namespace)
		var createdTopicTemplate *model.TopicTemplate
		createdTopicTemplate, err := cs.kafkaService.CreateTopicTemplate(ctx, topicTemplateFromReq)
		if err != nil {
			log.ErrorC(ctx, "error creating topic template with createdTopicTemplate: %v, %v", topicTemplateFromReq, err)
			return nil, err
		}
		log.InfoC(ctx, "Created topic template %v in %v namespace", createdTopicTemplate, namespace)
		result.CurrentSettings = createdTopicTemplate.GetSettings()
		result.PreviousSettings = nil
		return result, nil
	} else {
		log.InfoC(ctx, "Topic template with name %v and namespace %v was found. Checking if it has changed", topicTemplateFromReq, namespace)
		result.PreviousSettings = topicTemplate.GetSettings()
		result.CurrentSettings = topicTemplate.GetSettings() //changes only in case of full success

		if !topicTemplate.GetSettings().ReadOnlySettingsAreEqual(topicTemplateFromReq.GetSettings()) {
			return nil, errors.New("you're trying to change one of the read only settings. Operation is unsupported")
		}
		if topicTemplate.GetSettings().ConfigsAreEqual(topicTemplateFromReq.GetSettings()) {
			return result, nil
		}

		topics, err := cs.kafkaService.SearchTopicsInDB(ctx, &model.TopicSearchRequest{
			Template: topicTemplate.Id,
		})
		if err != nil {
			log.ErrorC(ctx, "error getting topics by template: %v, err: %v", topicTemplate, err)
			return nil, err
		}
		err = cs.kafkaService.MakeTopicsDirtyByTemplate(ctx, topicTemplate)
		if err != nil {
			log.ErrorC(ctx, "error making topics dirty by template: %v, err: %v", topicTemplate, err)
			return nil, err
		}

		var updatedTopics []model.TopicRegistrationRespDto
		for _, topic := range topics {
			topicResp, err := cs.kafkaService.UpdateTopicSettingsByTemplate(ctx, *topic, *topicTemplateFromReq)
			if err != nil {
				log.ErrorC(ctx, "error updating topic: %v by template: %v, err: %v", topic, topicTemplate, err)
				return result, err
			}
			updatedTopics = append(updatedTopics, *topicResp)
		}

		err = cs.kafkaService.UpdateTopicTemplate(ctx, *topicTemplate, *topicTemplateFromReq.GetSettings())
		if err != nil {
			log.ErrorC(ctx, "error updating topic template: %v, err: %v", topicTemplate, err)
			return result, err
		}

		result.UpdatedTopics = updatedTopics
		result.CurrentSettings = topicTemplateFromReq.GetSettings()
		return result, err
	}
}

func (cs *DefaultConfiguratorService) ApplyKafkaLazyTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	log.DebugC(ctx, "Lazy topic: %+v", cfg)
	topicDefinition, err := cs.applyKafkaTopicDefinition(ctx, cfg, namespace, model.TopicDefinitionKindLazy)
	if err != nil {
		log.ErrorC(ctx, "error getting or creating lazy topic with req: %v, %v", cfg, err)
		return nil, err
	}
	err = validator.Get().Var(&topicDefinition.Classifier, validator.ClassifierTag)
	if err != nil {
		return nil, fmt.Errorf(msg.InvalidClassifierFormat, topicDefinition.Classifier, err)
	}
	return topicDefinition.MakeTopicDefinitionResponse(), nil
}

func (cs *DefaultConfiguratorService) ApplyKafkaTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	log.InfoC(ctx, "Applying kafka tenant topic: %+v", cfg)
	tenantTopic, err := cs.applyKafkaTopicDefinition(ctx, cfg, namespace, model.TopicDefinitionKindTenant)
	if err != nil {
		log.ErrorC(ctx, "error getting or creating tenant topic with req: %v, %v", cfg, err)
		return nil, err
	}

	tenants, err := cs.tenantService.GetTenantsByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error getTenantsByNamespace with namespace: %v, error: %w", namespace, err)
	}

	responses := make([]model.SyncTenantsResp, 0)
	for _, tenant := range tenants {
		topicResp, err := cs.kafkaService.CreateTopicByTenantTopic(ctx, *tenantTopic, tenant)
		if err != nil {
			log.ErrorC(ctx, "error during CreateTopicByTenantTopic: %v", err)
			return responses, err
		}

		responses = append(responses, model.SyncTenantsResp{
			Tenant: tenant,
			Topics: []*model.TopicRegistrationRespDto{topicResp},
		})
	}

	return responses, nil
}

func (cs *DefaultConfiguratorService) applyKafkaTopicDefinition(ctx context.Context, cfg interface{}, namespace string, kind string) (*model.TopicDefinition, error) {
	config, ok := cfg.(*model.TopicRegistrationConfigReqDto)
	if !ok {
		err := fmt.Errorf("error during conversion to TopicRegistrationConfigReqDto for '%+v'", cfg)
		log.ErrorC(ctx, err.Error())
		return nil, err
	}

	topicDto := config.Spec
	log.InfoC(ctx, "Trying to get or create '%v' topic: %v", kind, topicDto)
	if err := validator.Get().Struct(&topicDto); err != nil {
		log.ErrorC(ctx, "Error in CheckFormat of classifier during applyKafkaTopicDefinition: %v", err)
		return nil, err
	}
	//TODO during migration we allow to use base or relative namespaces only for GET operations, later it should be with srv.dao.GetAllowedNamespaces(ctx, namespace)
	err := topicDto.Classifier.CheckAllowedNamespaces(namespace)
	if err != nil {
		log.ErrorC(ctx, "Error in VerifyAuth of classifier during applyKafkaTopicDefinition: %v", err)
		return nil, err
	}

	topicDefinition, err := topicDto.ToTopicDefinition(namespace, kind)
	if err != nil {
		log.ErrorC(ctx, "error converting to topic definition %s: %v", topicDto.Name, err)
		return nil, model.AggregateConfigError{
			Err:     model.ErrAggregateConfigParsing,
			Message: fmt.Sprintf("error converting to topic definition %s: %v", topicDto.Name, err),
		}
	}

	_, topicDefinition, err = cs.kafkaService.GetOrCreateTopicDefinition(ctx, topicDefinition)
	if err != nil {
		log.ErrorC(ctx, "error getting or creating '%v' topic with req: %v, %v", kind, topicDto, err)
		return nil, err
	}
	return topicDefinition, nil
}

func (cs *DefaultConfiguratorService) ApplyKafkaDeleteTopic(ctx context.Context, cfg interface{}, _ string) (interface{}, error) {
	return castAndExec(ctx, cfg, func(config *model.TopicDeleteConfig) (interface{}, error) {
		return cs.kafkaService.DeleteTopics(ctx, &model.TopicSearchRequest{
			Classifier:           config.Spec.Classifier,
			LeaveRealTopicIntact: config.Spec.LeaveRealTopicIntact,
		})
	})
}

func (cs *DefaultConfiguratorService) ApplyKafkaDeleteTenantTopic(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return castAndExec(ctx, cfg, func(config *model.TopicDeleteConfig) (interface{}, error) {
		_, err := cs.kafkaService.DeleteTopicDefinition(ctx, &config.Spec.Classifier)
		if err != nil {
			return nil, err
		}

		tenants, err := cs.tenantService.GetTenantsByNamespace(ctx, namespace)
		if err != nil {
			log.ErrorC(ctx, "error getTenantsByNamespace with namespace: %v, %v", namespace, err)
			return nil, err
		}

		responses := make([]*model.TopicDeletionResp, 0)
		for _, tenant := range tenants {
			tenantsClassifier := config.Spec.Classifier
			tenantsClassifier.TenantId = tenant.ExternalId
			topicResp, err := cs.kafkaService.DeleteTopics(ctx, &model.TopicSearchRequest{
				Classifier:           tenantsClassifier,
				LeaveRealTopicIntact: config.Spec.LeaveRealTopicIntact,
			})
			if err != nil {
				log.ErrorC(ctx, "error during DeleteTopics for tenant '%s': %v", tenant.ExternalId, err)
				return responses, err
			}

			responses = append(responses, topicResp)
		}
		return responses, nil
	})
}

func (cs *DefaultConfiguratorService) ApplyKafkaDeleteTopicTemplate(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	return castAndExec(ctx, cfg, func(config *model.TopicTemplateDeleteConfig) (interface{}, error) {
		return cs.kafkaService.DeleteTopicTemplate(ctx, config.Spec.Name, namespace)
	})
}

func castAndExec[T any](ctx context.Context, cfg any, exec func(v T) (interface{}, error)) (interface{}, error) {
	config, ok := cfg.(T)
	if !ok {
		return nil, utils.LogError(log, ctx, "error during conversion to %T for '%+v'", *new(T), cfg)
	}
	return exec(config)
}

func (cs *DefaultConfiguratorService) updateCompositeRegistration(ctx context.Context, namespace string, baseNamespace string) error {
	// search for registered composites on given base_namespace
	compositeRegistration, err := cs.registrationService.GetByNamespace(ctx, baseNamespace)
	if err != nil {
		return model.AggregateConfigError{
			Err:     fmt.Errorf("error getting composite registration by baseline: %v: %w", baseNamespace, err),
			Message: fmt.Sprintf("error getting composite registration by baseline: %v: %v", baseNamespace, err),
		}
	}

	if compositeRegistration == nil {
		// now we should correctly resolve compositeId.
		// In plain deployment compositeId is equals to aggregatedConfig.Spec.BaseNamespace that points to baseline namespace,
		// but in case baseline is BGDomain, then given aggregatedConfig.Spec.BaseNamespace value is points BGController namespace
		// So, in case of BGDomain we need to get name of origin namespace for composite id

		// NOTE! also we cannot rely on updated version of core-operator, that correctly maintain composite structure, so reimplement composite structure create here by hands
		log.InfoC(ctx, "No composite registration found by base namespace: `%v'. Search for BGDomain registration...", baseNamespace)

		domain, err := cs.bgDomain.FindByNamespace(ctx, baseNamespace)
		if err != nil {
			return model.AggregateConfigError{
				Err:     fmt.Errorf("error getting bgdomain registration by baseline: %v: %w", baseNamespace, err),
				Message: fmt.Sprintf("error getting bgdomain registration by baseline: %v: %v", baseNamespace, err),
			}
		}
		if domain != nil {
			log.InfoC(ctx, "Found BGDomain for baseline namespace `%v': %+v", baseNamespace, domain)
			// add all namespaces of BG Domain plus satellite namespace configuration is processing for
			compositeRegistration = &composite.CompositeRegistration{
				Id: domain.Origin,
				Namespaces: []string{
					domain.Origin,
					domain.ControllerNamespace,
					domain.Peer,
					namespace,
				}}
		} else {
			log.InfoC(ctx, "Create new composite with two plain members: [`%v', `%v']", baseNamespace, namespace)
			// this is plain (non BG) namespaces
			compositeRegistration = &composite.CompositeRegistration{
				Id: baseNamespace,
				Namespaces: []string{
					baseNamespace,
					namespace,
				}}
		}
	} else {
		log.InfoC(ctx, "Found composite registration: %+v", compositeRegistration)
		if slices.Contains(compositeRegistration.Namespaces, namespace) {
			log.InfoC(ctx, "Namespace `%v' already a member of composite. Skip alteration", namespace)
			return nil
		}

		compositeRegistration.Namespaces = append(compositeRegistration.Namespaces, namespace)
	}

	log.InfoC(ctx, "Update composite registration to: %+v", compositeRegistration)
	if err = cs.registrationService.Upsert(ctx, compositeRegistration); err != nil {
		return model.AggregateConfigError{
			Err:     model.ErrAggregateConfigBaseNamespace,
			Message: fmt.Sprintf("error during inserting base namespace '%v' for satellite namespace '%v'. error: %v", baseNamespace, namespace, err),
		}
	}

	return nil
}
