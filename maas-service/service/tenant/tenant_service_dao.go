package tenant

import (
	"context"
	"gorm.io/gorm"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/utils"
)

type TenantServiceDaoImpl struct {
	base dao.BaseDao
}

func NewTenantServiceDaoImpl(base dao.BaseDao) *TenantServiceDaoImpl {
	return &TenantServiceDaoImpl{base: base}
}

func (d *TenantServiceDaoImpl) InsertTenant(ctx context.Context, userTenant *model.Tenant) error {
	log.InfoC(ctx, "Inserting user tenant: %v", userTenant)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if e := cnn.Create(userTenant).Error; e != nil {
			if d.base.IsUniqIntegrityViolation(e) {
				log.InfoC(ctx, "Kafka userTenant was not inserted in db, because it already exists: %v", e.Error())
				return nil
			}
			return e
		}
		return nil
	}); err != nil {
		return utils.LogError(log, ctx, "error insert kafka userTenant in db: %w", err)
	}
	return nil
}

func (d *TenantServiceDaoImpl) GetTenantsByNamespace(ctx context.Context, namespace string) ([]model.Tenant, error) {
	log.InfoC(ctx, "GetTenantsByNamespace for namespace: %v", namespace)
	obj := new([]model.Tenant)

	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).
			Find(obj).Error
	})

	if err != nil {
		return nil, utils.LogError(log, ctx, "error GetTenantsByNamespace: %w", err)
	}

	return *obj, nil
}

func (d *TenantServiceDaoImpl) DeleteTenantsByNamespace(ctx context.Context, namespace string) error {
	log.InfoC(ctx, "DeleteTenantsByNamespace by namespace '%v'", namespace)
	obj := new(model.Tenant)

	err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("namespace=?", namespace).Delete(obj).Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error DeleteTenantsByNamespace: %v", err.Error())
		return err
	}

	return nil
}
