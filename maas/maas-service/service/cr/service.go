package cr

import (
	"context"
	"fmt"

	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"github.com/netcracker/qubership-maas/service/kafka"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/utils"
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
	resourceApplier *applier
	waitListDao     WaitListDao
	bgDomainService domain.BGDomainService
}

func NewCustomResourceProcessorService(waitListDao WaitListDao, configuratorService configurator_service.ConfiguratorService, kafkaService kafka.KafkaService, rabbitService rabbit_service.RabbitService, bgDomainService domain.BGDomainService) *CustomResourceProcessorService {
	return &CustomResourceProcessorService{
		waitListDao:     waitListDao,
		resourceApplier: newApplier(configuratorService, kafkaService, rabbitService, waitListDao),
		bgDomainService: bgDomainService,
	}
}

func (s *CustomResourceProcessorService) Apply(ctx context.Context, customResource *CustomResourceRequest, action Action) (*CustomResourceWaitEntity, error) {
	if err := s.checkNamespacePermission(ctx, customResource.Metadata.Namespace); err != nil {
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
	if err := s.checkNamespacePermission(ctx, waitEntity.Namespace); err != nil {
		return nil, err
	}

	return waitEntity, nil
}

func (s *CustomResourceProcessorService) Terminate(ctx context.Context, trackingId int64) (*CustomResourceWaitEntity, error) {
	waitEntity, err := s.waitListDao.UpdateStatus(ctx, trackingId, CustomResourceStatusTerminated)
	if err != nil {
		return nil, err
	}
	if err := s.checkNamespacePermission(ctx, waitEntity.Namespace); err != nil {
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

func (s *CustomResourceProcessorService) checkNamespacePermission(ctx context.Context, namespace string) error {
	sec := model.SecurityContextOf(ctx)
	if sec.UserHasAccessToNamespace(namespace) {
		return nil
	}

	account := sec.GetAccount()
	if account == nil {
		return fmt.Errorf("namespace '%s' is not allowed: %w", namespace, msg.AuthError)
	}

	namespaces, err := s.bgDomainService.FindByNamespace(ctx, account.Namespace)
	if err != nil {
		return fmt.Errorf("namespace '%s' is not allowed: %w", namespace, msg.AuthError)
	}
	if namespaces != nil && namespaces.IsMember(namespace) {
		return nil
	}

	return fmt.Errorf("namespace '%s' is not allowed: %w", namespace, msg.AuthError)
}
