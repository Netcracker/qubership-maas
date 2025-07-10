package instance

import (
	"context"
	"errors"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/utils/broadcast"
)

//go:generate mockgen -source=rabbit_instances_service.go -destination=mock/rabbit_instances_service.go
type RabbitInstanceService interface {
	Register(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error)
	GetRabbitInstances(ctx context.Context) (*[]model.RabbitInstance, error)

	GetById(ctx context.Context, id string) (*model.RabbitInstance, error)
	Update(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error)

	SetDefault(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error)
	GetDefault(ctx context.Context) (*model.RabbitInstance, error)

	Unregister(ctx context.Context, instanceId string) (*model.RabbitInstance, error)

	UpsertRabbitInstanceDesignator(ctx context.Context, instanceDesignator model.InstanceDesignatorRabbit, namespace string) error
	GetRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorRabbit, error)
	DeleteRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) error

	AddInstanceUpdateCallback(func(ctx context.Context, instance *model.RabbitInstance))
	CheckHealth(context.Context, *model.RabbitInstance) error
}

type RabbitInstanceServiceImpl struct {
	dao           RabbitInstancesDao
	broadcaster   *broadcast.Broadcast[*model.RabbitInstance]
	healthChecker func(rabbitInstance *model.RabbitInstance) error
}

func NewRabbitInstanceService(dao RabbitInstancesDao) RabbitInstanceService {
	return &RabbitInstanceServiceImpl{dao: dao, broadcaster: broadcast.New[*model.RabbitInstance](), healthChecker: helper.IsInstanceAvailable}
}

func NewRabbitInstanceServiceWithHealthChecker(dao RabbitInstancesDao, healthChecker func(rabbitInstance *model.RabbitInstance) error) RabbitInstanceService {
	return &RabbitInstanceServiceImpl{dao: dao, broadcaster: broadcast.New[*model.RabbitInstance](), healthChecker: healthChecker}
}

func (s *RabbitInstanceServiceImpl) AddInstanceUpdateCallback(cb func(ctx context.Context, instance *model.RabbitInstance)) {
	s.broadcaster.AddListener(cb)
}

func (s *RabbitInstanceServiceImpl) Register(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	if instance.GetId() == "" {
		instance.SetId(generateInstanceId())
		log.InfoC(ctx, "Instance id is not specified, generated new instance id: %v", instance.GetId())
	}

	if err := s.CheckHealth(ctx, instance); err != nil {
		return nil, utils.LogError(log, ctx, "error check instance health for: %+v: error: %s: %w", instance, err.Error(), msg.BadRequest)
	}

	return s.performOperation(ctx, instance, func() (*model.RabbitInstance, error) {
		log.InfoC(ctx, "Register rabbit instance with id = '%s'", instance.GetId())
		return s.dao.InsertInstanceRegistration(ctx, instance)
	})
}

func (s *RabbitInstanceServiceImpl) GetRabbitInstances(ctx context.Context) (*[]model.RabbitInstance, error) {
	return s.dao.GetRabbitInstanceRegistrations(ctx)
}

func (s *RabbitInstanceServiceImpl) GetById(ctx context.Context, id string) (*model.RabbitInstance, error) {
	result, err := s.dao.GetInstanceById(ctx, id)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during get instance by id: %w", err)
	}
	return result, nil
}

func (s *RabbitInstanceServiceImpl) Update(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Update rabbit instance with id = '%s'", instance.GetId())

	if err := s.CheckHealth(ctx, instance); err != nil {
		return nil, utils.LogError(log, ctx, "error check instance health for: %+v: error: %s: %w", instance, err.Error(), msg.BadRequest)
	}
	return s.performOperation(ctx, instance, func() (*model.RabbitInstance, error) {
		return s.dao.UpdateInstanceRegistration(ctx, instance)
	})
}

func (s *RabbitInstanceServiceImpl) SetDefault(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	return s.performOperation(ctx, instance, func() (*model.RabbitInstance, error) {
		return s.dao.SetDefaultInstance(ctx, instance)
	})
}

func (k *RabbitInstanceServiceImpl) GetDefault(ctx context.Context) (*model.RabbitInstance, error) {
	return k.dao.GetDefaultInstance(ctx)
}

func (s *RabbitInstanceServiceImpl) Unregister(ctx context.Context, instanceId string) (*model.RabbitInstance, error) {
	instance, err := s.dao.RemoveInstanceRegistration(ctx, instanceId)
	if err == nil {
		s.broadcaster.ExecuteListeners(ctx, instance)
		return instance, nil
	} else {
		return nil, err
	}
}

func (s *RabbitInstanceServiceImpl) UpsertRabbitInstanceDesignator(ctx context.Context, instanceDesignator model.InstanceDesignatorRabbit, namespace string) error {
	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		err := s.DeleteRabbitInstanceDesignatorByNamespace(ctx, namespace)
		if err != nil {
			log.ErrorC(ctx, "Error during DeleteRabbitInstanceDesignatorByNamespace before UpsertRabbitInstanceDesignator: %v", err)
			return err
		}
		return s.dao.InsertInstanceDesignatorRabbit(ctx, &instanceDesignator)
	})
}

func (s *RabbitInstanceServiceImpl) GetRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorRabbit, error) {
	return s.dao.GetRabbitInstanceDesignatorByNamespace(ctx, namespace)
}

func (s *RabbitInstanceServiceImpl) DeleteRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) error {
	return s.dao.DeleteRabbitInstanceDesignatorByNamespace(ctx, namespace)
}

func (s *RabbitInstanceServiceImpl) performOperation(ctx context.Context, instance model.MessageBrokerInstance, reg func() (*model.RabbitInstance, error)) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Perform operation with instance '%s'", instance.GetId())
	if instance.GetId() == "" {
		return nil, errors.New("Instance id not specified")
	}
	instance.FillDefaults()

	updated, err := reg()
	if err == nil {
		s.broadcaster.ExecuteListeners(ctx, updated)
		return updated, nil
	} else {
		return updated, err
	}
}

func (s *RabbitInstanceServiceImpl) CheckHealth(ctx context.Context, instance *model.RabbitInstance) error {
	return s.healthChecker(instance)
}
