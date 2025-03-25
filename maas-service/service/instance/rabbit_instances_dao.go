package instance

import (
	"context"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/utils"
)

//go:generate mockgen -source=rabbit_instances_dao.go -destination=mock/rabbit_instances_dao.go
type RabbitInstancesDao interface {
	InsertInstanceRegistration(context.Context, *model.RabbitInstance) (*model.RabbitInstance, error)
	GetRabbitInstanceRegistrations(context.Context) (*[]model.RabbitInstance, error)
	GetInstanceById(context.Context, string) (*model.RabbitInstance, error)
	UpdateInstanceRegistration(context.Context, *model.RabbitInstance) (*model.RabbitInstance, error)
	SetDefaultInstance(context.Context, *model.RabbitInstance) (*model.RabbitInstance, error)
	GetDefaultInstance(context.Context) (*model.RabbitInstance, error)
	RemoveInstanceRegistration(ctx context.Context, instanceId string) (*model.RabbitInstance, error)

	InsertInstanceDesignatorRabbit(ctx context.Context, instanceDesignator *model.InstanceDesignatorRabbit) error
	GetRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorRabbit, error)
	DeleteRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) error
}

type RabbitInstancesDaoImpl struct {
	base dao.BaseDao
}

func NewRabbitInstancesDao(base dao.BaseDao) RabbitInstancesDao {
	return &RabbitInstancesDaoImpl{base: base}
}

func (k RabbitInstancesDaoImpl) InsertInstanceRegistration(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Inserting instance registration: %+v", instance)
	updated := &model.RabbitInstance{}
	*updated = *instance // copy

	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			var registrationsNotFound bool
			if err := cnn.Model(model.RabbitInstance{}).Select("count(*) = 0").Find(&registrationsNotFound).Error; err != nil {
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
				if updated, err = k.SetDefaultInstance(ctx, instance); err != nil {
					return err
				}
			}

			return nil
		})
	})

	if err != nil {
		return nil, utils.LogError(log, ctx, "error inserting %s instance registration to db: %w", instance.BrokerType(), err)
	}

	return updated, err
}

func (k RabbitInstancesDaoImpl) GetRabbitInstanceRegistrations(ctx context.Context) (*[]model.RabbitInstance, error) {
	log.InfoC(ctx, "Querying registered rabbit instances")
	data := new([]model.RabbitInstance)

	if err := k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Find(data).Error
	}); err != nil {
		log.ErrorC(ctx, "Error selecting rabbit instances: %v", err.Error())
		return nil, err
	}
	return data, nil
}

func (k RabbitInstancesDaoImpl) GetInstanceById(ctx context.Context, id string) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Query rabbitmq instance by registration id: '%s'", id)
	instance := model.RabbitInstance{}

	if err := k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("id=?", id).First(&instance).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, utils.LogError(log, ctx, "error selecting rabbitmq instance with id '%s': %w", id, err)
	}
	return &instance, nil
}

func (k RabbitInstancesDaoImpl) UpdateInstanceRegistration(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		log.InfoC(ctx, "Update %s instance registration: %v", instance.BrokerType(), instance)
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			if exising, err := k.GetInstanceById(ctx, instance.GetId()); err != nil {
				return err
			} else {
				if exising == nil {
					return utils.LogError(log, ctx, "Can't update instance by non existing id `%v': %w", instance.GetId(), msg.BadRequest)
				} else {
					// user try to clean default flag on currently default instance
					if exising.Default && !instance.Default {
						return utils.LogError(log, ctx, "cancel instance update operation, because it leads to unspecified default"+
							" instance. Please change set `default' to true or set other instance registration as defaults: %w", msg.BadRequest)
					}
				}
			}

			if instance.IsDefault() {
				var err error
				if instance, err = k.SetDefaultInstance(ctx, instance); err != nil {
					return err
				}
			}

			if err := cnn.Save(instance).Error; err != nil {
				return utils.LogError(log, ctx, "error update %v instance: %w", instance, err)
			}

			return nil
		})
	})

	return instance, err
}

func (k RabbitInstancesDaoImpl) SetDefaultInstance(ctx context.Context, instance *model.RabbitInstance) (*model.RabbitInstance, error) {
	err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := cnn.Model(model.RabbitInstance{}).
			Where("is_default=true").
			Update("is_default", false).Error; err != nil {
			return fmt.Errorf("error clear is_default flag for instances type: %v, %w", instance.BrokerType(), err)
		}

		ret := cnn.Model(instance).Update("is_default", true)

		if ret.RowsAffected != 1 {
			return fmt.Errorf("no registered instance found by id: `%s'", instance.GetId())
		}

		return ret.Error
	})

	return instance, err
}

func (k RabbitInstancesDaoImpl) GetDefaultInstance(ctx context.Context) (*model.RabbitInstance, error) {
	data := model.RabbitInstance{}
	err := k.base.WithTx(ctx, func(_ context.Context, cnn *gorm.DB) error {
		return cnn.Where("is_default=true").Take(&data).Error
	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		log.WarnC(ctx, "no rabbitmq instance registered yet")
		return nil, nil
	case err != nil:
		return nil, utils.LogError(log, ctx, "unknown database error: %w", err)
	default:
		return &data, nil
	}
}

func (k RabbitInstancesDaoImpl) RemoveInstanceRegistration(ctx context.Context, instanceId string) (*model.RabbitInstance, error) {
	log.InfoC(ctx, "Remove rabbit instance by registration id: '%s'", instanceId)
	instance := &model.RabbitInstance{}
	err := dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			var err error
			instance, err = k.GetInstanceById(ctx, instanceId)
			if err != nil {
				return fmt.Errorf("error search instance `%v': %w", instance.GetId(), err)
			}
			if instance == nil {
				return fmt.Errorf("instance `%s' not found in database", instance.GetId())
			}

			registrations, err := k.GetRabbitInstanceRegistrations(ctx)
			if err != nil {
				return utils.LogError(log, ctx, "error count instances: %w", err)
			}
			count := len(*registrations)

			if instance.IsDefault() && count > 1 {
				return utils.LogError(log, ctx, "You're trying to remove default instance, while it is not last instance for broker."+
					" Please, at first set new default instance and then delete this instance. "+
					"Instances count: %v, current default instance id `%v': %w", count, instanceId, msg.BadRequest)
			} else {
				res := cnn.Delete(instance)
				switch {
				case res.Error != nil && k.base.IsForeignKeyIntegrityViolation(res.Error):
					return fmt.Errorf("unable to delete non empty rabbit instance `%v': %w", instance.GetId(), msg.BadRequest)
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
		return instance, utils.LogError(log, ctx, "Error removing %s instance by registration id: '%s' err: %w", instance.BrokerType(), instance.GetId(), err)
	}

	log.InfoC(ctx, "Instance was deleted successfully by registration id: %v", instance.GetId())
	return instance, nil
}

func (k RabbitInstancesDaoImpl) InsertInstanceDesignatorRabbit(ctx context.Context, instanceDesignator *model.InstanceDesignatorRabbit) error {
	log.InfoC(ctx, "Inserting instance designator rabbit to db: %+v", instanceDesignator)

	if err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "namespace"}},
			UpdateAll: true,
		}).Create(instanceDesignator).Error; e != nil {
			if k.base.IsUniqIntegrityViolation(e) {

				existingDesig, err := k.GetRabbitInstanceDesignatorByNamespace(ctx, instanceDesignator.Namespace)
				if err != nil {
					log.ErrorC(ctx, "Error getting rabbit instance designator by namespace during existing designator check: %v", err.Error())
					return err
				}

				err = dao.InstanceError{
					Err:     dao.InstanceDesignatorUniqueIndexErr,
					Message: fmt.Sprintf("Instance designator rabbit should be unique for namespace '%v', existing designator: '%+v'", instanceDesignator.Namespace, existingDesig),
				}
				log.ErrorC(ctx, e.Error())
				return err
			}
			if k.base.IsForeignKeyIntegrityViolation(e) {
				err := dao.InstanceError{
					Err:     dao.InstanceDesignatorForeignKeyErr,
					Message: fmt.Sprintf("instance designator rabbit should be linked to already existing instance, check your default instance '%v'", *instanceDesignator.DefaultInstanceId),
				}
				log.ErrorC(ctx, e.Error())
				return err
			}
			return e
		}

		return nil
	}); err != nil {
		log.ErrorC(ctx, "Error inserting instance designator rabbit to db: %v", err.Error())
		return err
	}

	return nil
}

func (k RabbitInstancesDaoImpl) GetRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) (*model.InstanceDesignatorRabbit, error) {
	log.InfoC(ctx, "Getting rabbit instance designator by namespace '%v'", namespace)
	obj := new(model.InstanceDesignatorRabbit)

	err := k.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).
			Preload("InstanceSelectors").
			Preload("DefaultInstance").
			Take(obj).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			log.InfoC(ctx, "Instance designator for rabbit was not found by namespace: %v", namespace)
			return nil, nil
		}
		log.ErrorC(ctx, "Error getting rabbit instance designator by namespace: %v", err.Error())
		return nil, err
	}

	//not very efficient, but gopg doesn't allow to get chain relations
	for _, selector := range obj.InstanceSelectors {
		instance, err := k.GetInstanceById(ctx, selector.InstanceId)
		if err != nil {
			log.ErrorC(ctx, "Error FindRabbitInstanceById, instance id '%v': %v", selector.InstanceId, err.Error())
			return nil, err
		}
		selector.Instance = instance
	}

	return obj, nil
}

func (k RabbitInstancesDaoImpl) DeleteRabbitInstanceDesignatorByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "Deleting rabbit instance designator by namespace '%v'", namespace)
	obj := new(model.InstanceDesignatorRabbit)

	err := k.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).Delete(obj).Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error deleting rabbit instance designator by namespace: %v", err.Error())
		return err
	}

	return nil
}
