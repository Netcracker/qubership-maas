package cr

import (
	"context"
	"fmt"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/service/configurator_service"
	"maas/maas-service/service/kafka"
	"maas/maas-service/service/rabbit_service"
	"maas/maas-service/utils"
)

type Action int

const (
	ActionCreate Action = iota
	ActionDelete
)

//go:generate mockgen -source=service.go -destination=service_mock.go -package cr
type WaitListDao interface {
	Create(ctx context.Context, customResourceWaitEntity *CustomResourceWaitEntity) error
	UpdateStatus(ctx context.Context, trackingId int64, status CustomResourceWaitStatus) (*CustomResourceWaitEntity, error)
	UpdateStatusAndReason(ctx context.Context, trackingId int64, status CustomResourceWaitStatus, reason string) (*CustomResourceWaitEntity, error)
	GetByTrackingId(ctx context.Context, trackingId int64) (*CustomResourceWaitEntity, error)
	FindByNamespaceAndStatus(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) ([]CustomResourceWaitEntity, error)
	DeleteByNamespace(ctx context.Context, namespace string) error
	DeleteByNameAndNamespace(ctx context.Context, name, namespace string) error
}

type CustomResourceProcessorService struct {
	resourceApplier     *applier
	configuratorService configurator_service.ConfiguratorService
	waitListDao         WaitListDao
}

func NewCustomResourceProcessorService(waitListDao WaitListDao, configuratorService configurator_service.ConfiguratorService, kafkaService kafka.KafkaService, rabbitService rabbit_service.RabbitService) *CustomResourceProcessorService {
	return &CustomResourceProcessorService{waitListDao: waitListDao, resourceApplier: newApplier(configuratorService, kafkaService, rabbitService, waitListDao)}
}

func (s *CustomResourceProcessorService) Apply(ctx context.Context, customResource *CustomResourceRequest, action Action) (*CustomResourceWaitEntity, error) {
	if err := checkNamespacePermission(ctx, customResource.Metadata.Namespace); err != nil {
		return nil, err
	}

	waitEntity, err := s.resourceApplier.apply(ctx, customResource, action)
	if err != nil {
		return nil, err
	}

	if waitEntity == nil {
		err = s.waitListDao.DeleteByNameAndNamespace(ctx, customResource.Metadata.Name, customResource.Metadata.Namespace)
		if err != nil {
			return nil, err
		}
	}

	return waitEntity, nil
}

func (s *CustomResourceProcessorService) GetStatus(ctx context.Context, trackingId int64) (*CustomResourceWaitEntity, error) {
	waitEntity, err := s.waitListDao.GetByTrackingId(ctx, trackingId)
	if err != nil {
		return nil, err
	}
	if waitEntity == nil {
		return nil, utils.LogError(log, ctx, "no entity found for trackingId '%d: %w", trackingId, msg.NotFound)
	}
	if err := checkNamespacePermission(ctx, waitEntity.Namespace); err != nil {
		return nil, err
	}

	return waitEntity, nil
}

func (s *CustomResourceProcessorService) Terminate(ctx context.Context, trackingId int64) (*CustomResourceWaitEntity, error) {
	waitEntity, err := s.waitListDao.UpdateStatus(ctx, trackingId, CustomResourceStatusTerminated)
	if err != nil {
		return nil, err
	}
	if err := checkNamespacePermission(ctx, waitEntity.Namespace); err != nil {
		return nil, err
	}

	return waitEntity, nil
}

func (s *CustomResourceProcessorService) CleanupNamespace(ctx context.Context, namespace string) error {
	if err := s.waitListDao.DeleteByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error cleanup namespace `%s': %w", namespace, err)
	}
	return nil
}

func checkNamespacePermission(ctx context.Context, namespace string) error {
	if !model.SecurityContextOf(ctx).UserHasAccessToNamespace(namespace) {
		return fmt.Errorf("namespace '%s' is not allowed: %w", namespace, msg.AuthError)
	}
	return nil
}
