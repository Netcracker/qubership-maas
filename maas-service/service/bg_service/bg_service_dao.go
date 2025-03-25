package bg_service

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"maas/maas-service/dao"
	"maas/maas-service/model"
)

//go:generate mockgen -source=bg_service_dao.go -destination=mock/bg_service_dao.go
type BgServiceDao interface {
	GetBgStatusByNamespace(context.Context, string) (*model.BgStatus, error)
	InsertBgStatus(ctx context.Context, cpMessage *model.BgStatus) error
	DeleteCpMessagesByNamespace(ctx context.Context, namespace string) error
	GetActiveVersionByNamespace(ctx context.Context, namespace string) (string, error)
}

type BgServiceDaoImpl struct {
	base dao.BaseDao
}

func NewBgServiceDao(base dao.BaseDao) BgServiceDao {
	return &BgServiceDaoImpl{base: base}
}

func (d *BgServiceDaoImpl) GetActiveVersionByNamespace(ctx context.Context, namespace string) (string, error) {
	cpMessage := new(model.BgStatus)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.
			Where("namespace=?", namespace).
			Order("timestamp DESC").
			First(cpMessage).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", nil
		}
		log.ErrorC(ctx, "Error during GetActiveVersionByNamespace in db: %v", err.Error())
		return "", err
	}
	return cpMessage.Active, nil
}

func (d *BgServiceDaoImpl) InsertBgStatus(ctx context.Context, cpMessage *model.BgStatus) error {
	log.InfoC(ctx, "Insert bg status to db: %+v", cpMessage)
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Create(cpMessage).Error
	}); err != nil {
		log.ErrorC(ctx, "Error insert entity to db: %v", err.Error())
		return err
	}

	return nil
}

func (d *BgServiceDaoImpl) GetBgStatusByNamespace(ctx context.Context, namespace string) (*model.BgStatus, error) {
	cpMessage := new(model.BgStatus)
	if err := d.base.UsingDb(ctx, func(conn *gorm.DB) error {
		return conn.
			Where("namespace=?", namespace).
			Order("timestamp DESC").
			First(cpMessage).Error
	}); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		log.ErrorC(ctx, "Error during search cp messages by namespace=%s: %+v", namespace, err)
		return nil, err
	}
	return cpMessage, nil
}

func (d *BgServiceDaoImpl) DeleteCpMessagesByNamespace(ctx context.Context, namespace string) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.
			Where("namespace=?", namespace).
			Delete(&model.BgStatus{}).Error
	}); err != nil {
		log.ErrorC(ctx, "Error deleting cpMessages by namespace in db: %v", err.Error())
		return err
	}
	return nil
}
