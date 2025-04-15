package instance

import (
	"context"
	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	kafka_helper "github.com/netcracker/qubership-maas/service/kafka/helper"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/utils/broadcast"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("instance")
}

//go:generate mockgen -source=kafka_instances_service.go -destination=mock/kafka_instances_service.go
type KafkaInstanceService interface {
	Register(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error)

	GetKafkaInstances(ctx context.Context) (*[]model.KafkaInstance, error)

	GetById(ctx context.Context, id string) (*model.KafkaInstance, error)
	Update(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error)

	SetDefault(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error)
	GetDefault(ctx context.Context) (*model.KafkaInstance, error)

	Unregister(ctx context.Context, instanceId string) (*model.KafkaInstance, error)

	UpsertKafkaInstanceDesignator(ctx context.Context, instanceDesignator model.InstanceDesignatorKafka, namespace string) error
	GetKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorKafka, error)
	DeleteKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) error

	AddInstanceUpdateCallback(func(ctx context.Context, instance *model.KafkaInstance))
	CheckHealth(context.Context, *model.KafkaInstance) error
}

type KafkaInstanceServiceImpl struct {
	dao           KafkaInstancesDao
	helper        kafka_helper.Helper
	broadcaster   *broadcast.Broadcast[*model.KafkaInstance]
	healthChecker func(kafkaInstance *model.KafkaInstance) error
}

func NewKafkaInstanceService(dao KafkaInstancesDao, helper kafka_helper.Helper) KafkaInstanceService {
	return &KafkaInstanceServiceImpl{dao: dao, helper: helper, broadcaster: broadcast.New[*model.KafkaInstance](), healthChecker: kafka_helper.IsInstanceAvailable}
}

func NewKafkaInstanceServiceWithHealthChecker(dao KafkaInstancesDao, helper kafka_helper.Helper, healthChecker func(kafkaInstance *model.KafkaInstance) error) KafkaInstanceService {
	return &KafkaInstanceServiceImpl{dao: dao, helper: helper, broadcaster: broadcast.New[*model.KafkaInstance](), healthChecker: healthChecker}
}

func (s *KafkaInstanceServiceImpl) AddInstanceUpdateCallback(cb func(ctx context.Context, instance *model.KafkaInstance)) {
	s.broadcaster.AddListener(cb)
}

func (s *KafkaInstanceServiceImpl) Register(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	if instance.GetId() == "" {
		instance.SetId(generateInstanceId())
		log.InfoC(ctx, "Instance id is not specified, generated new instance id: %v", instance.GetId())
	}

	if err := s.CheckHealth(ctx, instance); err != nil {
		return nil, utils.LogError(log, ctx, "unable to check health of instance: %+v, %w", instance, msg.BadRequest)
	}

	return s.performOperation(ctx, instance, func() (*model.KafkaInstance, error) {
		return s.dao.InsertInstanceRegistration(ctx, instance)
	})
}

func (s *KafkaInstanceServiceImpl) GetKafkaInstances(ctx context.Context) (*[]model.KafkaInstance, error) {
	return s.dao.GetKafkaInstanceRegistrations(ctx)
}

func (s *KafkaInstanceServiceImpl) GetById(ctx context.Context, id string) (*model.KafkaInstance, error) {
	result, err := s.dao.GetInstanceById(ctx, id)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during get instance by id: %w", err)
	}
	return result, nil
}

func (s *KafkaInstanceServiceImpl) Update(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Update kafka instance with id = '%s'", instance.GetId())
	if err := s.CheckHealth(ctx, instance); err != nil {
		return nil, utils.LogError(log, ctx, "unable to check health of instance: %+v, %w", instance, msg.BadRequest)
	}
	return s.performOperation(ctx, instance, func() (*model.KafkaInstance, error) {
		return s.dao.UpdateInstanceRegistration(ctx, instance)
	})
}

func (s *KafkaInstanceServiceImpl) SetDefault(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	return s.performOperation(ctx, instance, func() (*model.KafkaInstance, error) {
		updated, err := s.dao.SetDefaultInstance(ctx, instance)
		return updated, err
	})
}

func (s *KafkaInstanceServiceImpl) GetDefault(ctx context.Context) (*model.KafkaInstance, error) {
	return s.dao.GetDefaultInstance(ctx)
}

func (s *KafkaInstanceServiceImpl) Unregister(ctx context.Context, instanceId string) (*model.KafkaInstance, error) {
	instance, err := s.dao.RemoveInstanceRegistration(ctx, instanceId)
	if err == nil {
		s.broadcaster.ExecuteListeners(ctx, instance)
		return instance, nil
	} else {
		return nil, err
	}
}

func (s *KafkaInstanceServiceImpl) UpsertKafkaInstanceDesignator(ctx context.Context, instanceDesignator model.InstanceDesignatorKafka, namespace string) error {
	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		err := s.DeleteKafkaInstanceDesignatorByNamespace(ctx, namespace)
		if err != nil {
			log.ErrorC(ctx, "Error during DeleteKafkaInstanceDesignatorByNamespace before UpsertKafkaInstanceDesignator: %v", err)
			return err
		}
		return s.dao.InsertInstanceDesignatorKafka(ctx, &instanceDesignator)
	})
}

func (s *KafkaInstanceServiceImpl) GetKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorKafka, error) {
	return s.dao.GetKafkaInstanceDesignatorByNamespace(ctx, namespace)
}

func (s *KafkaInstanceServiceImpl) DeleteKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Delete designators for: %v", namespace)
	if err := s.dao.DeleteKafkaInstanceDesignatorByNamespace(ctx, namespace); err != nil {
		return utils.LogError(log, ctx, "error deleting instance designators: %w", err)
	}
	return nil
}

func (s *KafkaInstanceServiceImpl) performOperation(ctx context.Context, instance *model.KafkaInstance, reg func() (*model.KafkaInstance, error)) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Perform operation with instance '%s'", instance.GetId())
	if instance.GetId() == "" {
		return nil, errors.New("Instance id not specified")
	}
	instance.FillDefaults()

	updated, err := reg()
	if err == nil {
		s.broadcaster.ExecuteListeners(ctx, instance)
		return updated, nil
	} else {
		return nil, err
	}
}

func generateInstanceId() string {
	return uuid.New().String()
}

func MatchDesignator(ctx context.Context, classifier model.Classifier, designator model.InstanceDesignator, domainNamespaceSupplier func(namespace string) string) (any, bool, model.ClassifierMatch) {
	for _, selector := range designator.GetInstanceSelectors() {
		allMatch := true

		clMatch := selector.GetClassifierMatch()
		if clMatch.Name != "" && (clMatch.Name != classifier.Name && !utils.MatchPattern(ctx, clMatch.Name, classifier.Name)) {
			allMatch = false
		}

		namespace := domainNamespaceSupplier(classifier.Namespace)
		if clMatch.Namespace != "" && clMatch.Namespace != namespace && !utils.MatchPattern(ctx, clMatch.Namespace, namespace) {
			allMatch = false
		}

		if clMatch.TenantId != "" && (clMatch.TenantId != classifier.TenantId && !utils.MatchPattern(ctx, clMatch.TenantId, classifier.TenantId)) {
			allMatch = false
		}

		if allMatch {
			return selector.GetInstance(), true, selector.GetClassifierMatch()
		}
	}
	return nil, false, model.ClassifierMatch{}
}

func (s *KafkaInstanceServiceImpl) CheckHealth(ctx context.Context, instance *model.KafkaInstance) error {
	return s.healthChecker(instance)
}
