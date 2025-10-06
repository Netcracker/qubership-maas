package cr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/mitchellh/mapstructure"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"github.com/netcracker/qubership-maas/service/kafka"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/validator"
	"strings"
)

type kindHandler func(context.Context, *CustomResourceSpecRequest, *CustomResourceMetadataRequest) (any, error)

type registry map[kind]kindHandler

type applier struct {
	registry    registry
	waitListDao WaitListDao
}

type kind struct {
	ApiVersion string
	SubKind    string
	Action     Action
}

func add[T any](r registry, key kind, f func(ctx context.Context, config *T, metadata *CustomResourceMetadataRequest) (any, error)) {
	r[key] = func(ctx context.Context, spec *CustomResourceSpecRequest, metadata *CustomResourceMetadataRequest) (any, error) {
		var arg T
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			ErrorUnused: true,
			Result:      &arg,
		})
		if err != nil {
			return nil, fmt.Errorf("decode error: %v: %w", err, msg.BadRequest)
		}
		err = decoder.Decode(spec)
		if err != nil {
			var mapstructureErr *mapstructure.Error
			errorMsg := err.Error()
			if errors.As(err, &mapstructureErr) {
				errorMsg = strings.Join(mapstructureErr.Errors, "; ")
			}
			return nil, utils.LogError(log, ctx, "can not parse yaml: %s. err: %w", errorMsg, msg.BadRequest)

		}
		if err := validator.Get().StructCtx(ctx, arg); err != nil {
			return nil, utils.LogError(log, ctx, "error validate input: %s: %w", err.Error(), msg.BadRequest)
		}
		return f(ctx, &arg, metadata)
	}
}

func newApplier(configuratorService configurator_service.ConfiguratorService, kafkaService kafka.KafkaService, rabbitService rabbit_service.RabbitService, waitListDao WaitListDao) *applier {
	registry := make(map[kind]kindHandler)
	kafkaHandler := NewKafkaHandler(configuratorService, kafkaService)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTopic, Action: ActionCreate}, kafkaHandler.HandleTopicConfiguration)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTopicTemplate, Action: ActionCreate}, kafkaHandler.HandleTopicTemplateConfiguration)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindLazyTopic, Action: ActionCreate}, kafkaHandler.HandleLazyTopicConfiguration)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTenantTopic, Action: ActionCreate}, kafkaHandler.HandleTenantTopicConfiguration)

	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTopic, Action: ActionDelete}, kafkaHandler.HandleDeleteTopicConfiguration)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTopicTemplate, Action: ActionDelete}, kafkaHandler.HandleDeleteTopicTemplateConfiguration)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindTenantTopic, Action: ActionDelete}, kafkaHandler.HandleDeleteTenantTopicConfiguration)

	rabbitHandler := NewRabbitHandler(configuratorService, rabbitService)
	add(registry, kind{ApiVersion: configurator_service.CoreNcV1ApiVersion, SubKind: configurator_service.CustomResourceKindVhost, Action: ActionCreate}, rabbitHandler.HandleVhostConfiguration)

	return &applier{
		registry:    registry,
		waitListDao: waitListDao,
	}
}

func (a applier) apply(ctx context.Context, customResource *CustomResourceRequest, action Action) (*CustomResourceWaitEntity, error) {
	kHandler, err := a.getHandlerForCustomResource(customResource, action)
	if err != nil {
		return nil, err
	}
	namespace := customResource.Metadata.Namespace
	_, err = kHandler(ctx, customResource.Spec, customResource.Metadata)
	if err != nil {
		if errors.Is(err, ErrDependencyNotFound) {
			customResourceWaitEntity, err := a.putToWaitList(ctx, customResource, namespace, err.Error())
			if err != nil {
				return nil, err
			}
			return customResourceWaitEntity, nil
		}
		return nil, err
	}
	err = a.applyFromWaitList(ctx, namespace)
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func (a applier) putToWaitList(ctx context.Context, customResource *CustomResourceRequest, namespace, reason string) (*CustomResourceWaitEntity, error) {
	customResourceJson, err := json.Marshal(customResource)
	if err != nil {
		return nil, err
	}
	customResourceWaitEntity := CustomResourceWaitEntity{
		CustomResource: string(customResourceJson),
		Namespace:      namespace,
		Status:         CustomResourceStatusInProgress,
		Reason:         reason,
	}
	err = a.waitListDao.Create(ctx, &customResourceWaitEntity)
	if err != nil {
		return nil, err
	}
	return &customResourceWaitEntity, nil
}

func (a applier) getHandlerForCustomResource(customResource *CustomResourceRequest, action Action) (kindHandler, error) {
	k := kind{
		ApiVersion: customResource.ApiVersion,
		SubKind:    customResource.SubKind,
		Action:     action,
	}

	kHandler, ok := a.registry[k]
	if !ok {
		return nil, fmt.Errorf("unknown kind '%v': %w", k, msg.BadRequest)
	}
	return kHandler, nil
}

func (a applier) applyFromWaitList(ctx context.Context, namespace string) error {
	customResourceWaits, err := a.waitListDao.FindByNamespaceAndStatus(ctx, namespace, CustomResourceStatusInProgress)
	if err != nil {
		return err
	}
	for _, customResourceWait := range customResourceWaits {
		var cr CustomResourceRequest
		err := json.Unmarshal([]byte(customResourceWait.CustomResource), &cr)
		if err != nil {
			return utils.LogError(log, ctx, "can not parse json: %s: %w", err.Error(), msg.BadRequest)
		}
		kHandler, err := a.getHandlerForCustomResource(&cr, ActionCreate)
		if err != nil {
			return err
		}
		_, err = kHandler(ctx, cr.Spec, cr.Metadata)
		if err != nil {
			if errors.Is(err, ErrDependencyNotFound) {
				continue // leave in wait list
			}
			if _, err = a.waitListDao.UpdateStatusAndReason(ctx, customResourceWait.TrackingId, CustomResourceStatusFailed, err.Error()); err != nil {
				return err
			}
		}
		if _, err = a.waitListDao.UpdateStatus(ctx, customResourceWait.TrackingId, CustomResourceStatusCompleted); err != nil {
			return err
		}
	}
	return nil
}
