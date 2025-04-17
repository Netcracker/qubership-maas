package instance

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

//go:generate mockgen -source=kafka_instances_dao.go -destination=mock/kafka_instances_dao.go
type KafkaInstancesDao interface {
	InsertInstanceRegistration(context.Context, *model.KafkaInstance) (*model.KafkaInstance, error)
	GetKafkaInstanceRegistrations(context.Context) (*[]model.KafkaInstance, error)
	GetInstanceById(context.Context, string) (*model.KafkaInstance, error)
	UpdateInstanceRegistration(context.Context, *model.KafkaInstance) (*model.KafkaInstance, error)
	SetDefaultInstance(context.Context, *model.KafkaInstance) (*model.KafkaInstance, error)
	GetDefaultInstance(context.Context) (*model.KafkaInstance, error)
	RemoveInstanceRegistration(ctx context.Context, instanceId string) (*model.KafkaInstance, error)

	InsertInstanceDesignatorKafka(ctx context.Context, instanceDesignator *model.InstanceDesignatorKafka) error
	GetKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorKafka, error)
	DeleteKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) error
}

type KafkaInstancesDaoImpl struct {
	base dao.BaseDao
	bg   domain.BGDomainDao
}

func NewKafkaInstancesDao(base dao.BaseDao, bg domain.BGDomainDao) KafkaInstancesDao {
	return &KafkaInstancesDaoImpl{base: base, bg: bg}
}

func (k KafkaInstancesDaoImpl) InsertInstanceRegistration(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Inserting instance registration: %+v", instance)

	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			var registrationsNotFound bool
			if err := cnn.Model(model.KafkaInstance{}).Select("count(*) = 0").Find(&registrationsNotFound).Error; err != nil {
				return err
			}

			if err := cnn.Create(instance).Error; err != nil {
				if k.base.IsUniqIntegrityViolation(err) {
					return utils.LogError(log, ctx, "constraint violation: Instance with id=%s already registered: %w", instance.GetId(), msg.BadRequest)
				}
				return err
			}

			// this is new default instance
			if instance.IsDefault() || registrationsNotFound {
				var err error
				if instance, err = k.SetDefaultInstance(ctx, instance); err != nil {
					return err
				}
			}

			return nil
		})
	})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error inserting %s instance registration to db: %w", instance.BrokerType(), err)
	}

	return instance, nil
}

func (k KafkaInstancesDaoImpl) GetKafkaInstanceRegistrations(ctx context.Context) (*[]model.KafkaInstance, error) {
	log.InfoC(ctx, "Querying registered kafka instances")
	data := new([]model.KafkaInstance)

	if err := k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Find(data).Error
	}); err != nil {
		log.ErrorC(ctx, "Error selecting kafka instances: %v", err.Error())
		return nil, err
	}
	return data, nil
}

func (k KafkaInstancesDaoImpl) GetInstanceById(ctx context.Context, id string) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Query kafka instance by registration id: '%s'", id)
	instance := model.KafkaInstance{}

	if err := k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("id=?", id).First(&instance).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, utils.LogError(log, ctx, "error selecting kafka instance with id '%s': %w", id, err)
	}
	return &instance, nil
}

func (k KafkaInstancesDaoImpl) UpdateInstanceRegistration(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		log.InfoC(ctx, "Update %s instance registration: %v", instance.BrokerType(), instance)
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			if exising, err := k.GetInstanceById(ctx, instance.GetId()); err != nil {
				return err
			} else {
				if exising == nil {
					return utils.LogError(log, ctx, "can't update instance by non existing id `%v': %w", instance.GetId(), msg.NotFound)
				} else {
					// user try to clean default flag on currently default instance
					if exising.Default && !instance.Default {
						return utils.LogError(log, ctx, "cancel instance update operation, because it leads to unspecified default"+
							" instance. Please change set `default' to true or set other instance registration as defaults: %w", msg.BadRequest)
					}
				}
			}

			if err := cnn.Save(instance).Error; err != nil {
				return utils.LogError(log, ctx, "error update %v instance: %w", instance, err)
			}

			if instance.IsDefault() {
				if _, err := k.SetDefaultInstance(ctx, instance); err != nil {
					return utils.LogError(log, ctx, "error setting default instance: %w", err)
				}
			}

			return nil
		})
	})

	return instance, err
}

func (k KafkaInstancesDaoImpl) SetDefaultInstance(ctx context.Context, instance *model.KafkaInstance) (*model.KafkaInstance, error) {
	err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := cnn.Model(model.KafkaInstance{}).
			Where("is_default=true").
			Update("is_default", false).Error; err != nil {
			return fmt.Errorf("error clear is_default flag for instances type: %v, %w", instance.BrokerType(), err)
		}

		ret := cnn.Model(instance).Clauses(clause.Returning{}).Update("is_default", true)

		if ret.RowsAffected != 1 {
			return fmt.Errorf("no registered instance found by id: `%s'", instance.GetId())
		}

		return ret.Error
	})

	return instance, err
}

func (k KafkaInstancesDaoImpl) GetDefaultInstance(ctx context.Context) (*model.KafkaInstance, error) {
	data := model.KafkaInstance{}
	err := k.base.WithTx(ctx, func(_ context.Context, cnn *gorm.DB) error {
		return cnn.Where("is_default=true").Take(&data).Error
	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		log.WarnC(ctx, "no kafka instance registered yet")
		return nil, nil
	case err != nil:
		return nil, utils.LogError(log, ctx, "unknown database error: %w", err)
	default:
		return &data, nil
	}
}

func (k KafkaInstancesDaoImpl) RemoveInstanceRegistration(ctx context.Context, instanceId string) (*model.KafkaInstance, error) {
	log.InfoC(ctx, "Remove kafka instance by registration id: '%s'", instanceId)
	instance := &model.KafkaInstance{}
	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			var err error
			instance, err = k.GetInstanceById(ctx, instanceId)
			if err != nil {
				return fmt.Errorf("error search instance `%v': %w", instanceId, err)
			}
			if instance == nil {
				return fmt.Errorf("instance `%s' not found in database: %w", instanceId, msg.NotFound)
			}

			registrations, err := k.GetKafkaInstanceRegistrations(ctx)
			if err != nil {
				return utils.LogError(log, ctx, "error count instances: %w", err)
			}
			count := len(*registrations)

			// we permit deletion default instance only if its instance is last in database
			if instance.IsDefault() && count > 1 {
				return utils.LogError(log, ctx, "You're trying to remove default instance, while it is not last instance for broker."+
					" Please, at first set new default instance and then delete this instance. "+
					"Instances count: %v, current default instance id `%v': %w", count, instanceId, msg.BadRequest)
			} else {
				res := cnn.Delete(instance)
				switch {
				case res.Error != nil && k.base.IsForeignKeyIntegrityViolation(res.Error):
					return fmt.Errorf("unable to delete non empty kafka instance `%v': %w", instance.GetId(), msg.BadRequest)
				case res.Error != nil:
					return fmt.Errorf("error delete instance `%v' from database: %w", instance.GetId(), res.Error)
				case res.RowsAffected != 1:
					return fmt.Errorf("instance `%s' was not deleted from database by unknown reason", instance.GetId())
				default:
					return nil
				}
			}
		})
	})

	if err != nil {
		if instance == nil {
			return nil, utils.LogError(log, ctx, "Error removing instance by registration id: '%s' err: %w", instanceId, err)
		} else {
			return instance, utils.LogError(log, ctx, "Error removing %s instance by registration id: '%s' err: %w", instance.BrokerType(), instance.GetId(), err)
		}
	}

	log.InfoC(ctx, "Instance was deleted successfully by registration id: %v", instance.GetId())
	return instance, nil
}

func (k KafkaInstancesDaoImpl) InsertInstanceDesignatorKafka(ctx context.Context, instanceDesignator *model.InstanceDesignatorKafka) error {
	log.InfoC(ctx, "Inserting instance designator kafka to db: %+v", instanceDesignator)

	domainEntity, err := k.bg.FindByNamespace(ctx, instanceDesignator.Namespace)
	if err != nil {
		log.ErrorC(ctx, "can not get namespace domain for '%s'; use designator's instance : %w")
	}
	if domainEntity != nil {
		if instanceDesignator.Namespace == domainEntity.Peer {
			instanceDesignator.Namespace = domainEntity.Origin
		}

		for i, selector := range instanceDesignator.InstanceSelectors {
			if selector.ClassifierMatch.Namespace == domainEntity.Peer {
				instanceDesignator.InstanceSelectors[i].ClassifierMatch.Namespace = domainEntity.Origin
			}
		}
	}

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		if err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			if e := cnn.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "namespace"}},
				UpdateAll: true,
			}).Create(instanceDesignator).Error; e != nil {
				if k.base.IsUniqIntegrityViolation(e) {
					existingDesig, err := k.GetKafkaInstanceDesignatorByNamespace(ctx, instanceDesignator.Namespace)
					if err != nil {
						log.ErrorC(ctx, "Error getting kafka instance designator by namespace during existing designator check: %v", err.Error())
						return err
					}

					err = dao.InstanceError{
						Err:     dao.InstanceDesignatorUniqueIndexErr,
						Message: fmt.Sprintf("Instance designator should be unique for namespace '%v', existing designator: '%+v'", instanceDesignator.Namespace, existingDesig),
					}
					log.ErrorC(ctx, e.Error())
					return err
				}
				if k.base.IsForeignKeyIntegrityViolation(e) {
					if pgError, ok := e.(*pgconn.PgError); ok {
						var err error
						if pgError.TableName == (&model.InstanceSelectorKafka{}).TableName() {
							err = dao.InstanceError{
								Err:     dao.InstanceDesignatorForeignKeyErr,
								Message: fmt.Sprintf("instance designator should be linked to already existing instance, check your selector instance: %s", pgError.Detail),
							}
						} else if pgError.TableName == (&model.InstanceDesignatorKafka{}).TableName() {
							err = dao.InstanceError{
								Err:     dao.InstanceDesignatorForeignKeyErr,
								Message: fmt.Sprintf("instance designator should be linked to already existing instance, check your default instance '%v'", *instanceDesignator.DefaultInstanceId),
							}
						}
						log.ErrorC(ctx, e.Error())
						return err
					} else {
						log.PanicC(ctx, "Unexpected error type")
					}
				}
				return e
			}

			return nil
		}); err != nil {
			log.ErrorC(ctx, "Error inserting instance designator kafka to db: %v", err.Error())
			return err
		}

		return nil
	})
}

func (k KafkaInstancesDaoImpl) GetKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorKafka, error) {
	log.InfoC(ctx, "Getting kafka instance designator by namespace '%v'", namespace)
	obj := new(model.InstanceDesignatorKafka)
	domainEntity, err := k.bg.FindByNamespace(ctx, namespace)
	if err != nil {
		return nil, err
	}
	if domainEntity != nil {
		namespace = domainEntity.Origin
	}
	err = k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("namespace = ?", namespace).
			Preload("InstanceSelectors").
			Preload("DefaultInstance").
			Take(obj).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.InfoC(ctx, "Instance designator for kafka was not found by namespace: %v", namespace)
			return nil, nil
		}
		log.ErrorC(ctx, "Error getting kafka instance designator by namespace: %v", err.Error())
		return nil, err
	}

	//not very efficient, but gopg doesn't allow to get chain relations
	for _, selector := range obj.InstanceSelectors {
		instance, err := k.GetInstanceById(ctx, selector.InstanceId)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error getting instance by id `%v': %w", selector.InstanceId, err)
		}
		if instance == nil {
			return nil, utils.LogError(log, ctx, "instance not found by id `%v': %w", selector.InstanceId, msg.NotFound)
		}

		selector.Instance = instance
	}

	return obj, nil
}

func (k KafkaInstancesDaoImpl) DeleteKafkaInstanceDesignatorByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Deleting kafka instance designator by namespace '%v'", namespace)
	obj := new(model.InstanceDesignatorKafka)

	err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).Delete(obj).Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error deleting kafka instance designator by namespace: %v", err.Error())
		return err
	}

	return nil
}
