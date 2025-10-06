package cr

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("custom_resource")
}

type CustomResourceWaitStatus string
type CustomResourceType string

type CustomResourceWaitEntity struct {
	TrackingId     int64 `gorm:"primaryKey"`
	CustomResource string
	Namespace      string
	Reason         string
	Status         CustomResourceWaitStatus
	CreatedAt      time.Time
}

func (_ CustomResourceWaitEntity) TableName() string {
	return "custom_resource_waits"
}

type PGWaitListDao struct {
	base dao.BaseDao
}

func NewPGWaitListDao(base dao.BaseDao) *PGWaitListDao {
	return &PGWaitListDao{base: base}
}

func (d *PGWaitListDao) FindByNamespaceAndStatus(ctx context.Context, namespace string, status ...CustomResourceWaitStatus) ([]CustomResourceWaitEntity, error) {
	res := make([]CustomResourceWaitEntity, 0)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("namespace = ? and status in ?", namespace, status).Find(&res).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error finding CustomResourceWaitEntities by namespace '%s': %w", namespace, err)
	}
	return res, nil
}

func (d *PGWaitListDao) GetByTrackingId(ctx context.Context, trackingId int64) (*CustomResourceWaitEntity, error) {
	var res *CustomResourceWaitEntity
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		result := cnn.Where("tracking_id = ?", trackingId).Find(&res)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			res = nil
		}
		return nil
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error getting CustomResourceWait in db: %w", err)
	}
	return res, nil
}

func (d *PGWaitListDao) Create(ctx context.Context, customResourceWaitEntity *CustomResourceWaitEntity) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		res := cnn.Clauses(clause.OnConflict{DoNothing: true}).Create(customResourceWaitEntity)
		if err := res.Error; err != nil {
			return utils.LogError(log, ctx, "error insert CustomResourceWaitEntity: %v, error: %w", customResourceWaitEntity, err)
		}
		if res.RowsAffected == 0 {
			result := cnn.Where("custom_resource = ?", customResourceWaitEntity.CustomResource).Find(&customResourceWaitEntity)
			if result.Error != nil {
				return result.Error
			}
		}
		return nil
	}); err != nil {
		return utils.LogError(log, ctx, "error creating CustomResourceWait in db: %w", err)
	}
	return nil
}

func (d *PGWaitListDao) UpdateStatusAndReason(ctx context.Context, trackingId int64, status CustomResourceWaitStatus, reason string) (*CustomResourceWaitEntity, error) {
	var res CustomResourceWaitEntity
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Model(&res).
			Where("tracking_id = ?", trackingId).
			Clauses(clause.Returning{}).
			Updates(map[string]interface{}{"status": status, "reason": reason})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("no entity found by trackingId '%d': %w ", trackingId, msg.BadRequest)
		}
		return nil
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error update status by trackingId : %d, error: %w", trackingId, err)
	}
	return &res, nil
}

func (d *PGWaitListDao) UpdateStatus(ctx context.Context, trackingId int64, status CustomResourceWaitStatus) (*CustomResourceWaitEntity, error) {
	var res CustomResourceWaitEntity
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		result := cnn.Model(&res).
			Where("tracking_id = ?", trackingId).
			Clauses(clause.Returning{}).
			Update("status", status)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("no entity found by trackingId '%d': %w ", trackingId, msg.BadRequest)
		}
		return nil
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error update status by trackingId : %d, error: %w", trackingId, err)
	}
	return &res, nil
}

func (d *PGWaitListDao) DeleteByNamespace(ctx context.Context, namespace string) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := cnn.Where("namespace = ?", namespace).Delete(&CustomResourceWaitEntity{}).Error; err != nil {
			return utils.LogError(log, ctx, "error delete CustomResourceWaitEntities by namespace: %s, error: %w", namespace, err)
		}
		return nil
	}); err != nil {
		return utils.LogError(log, ctx, "error deleting CustomResourceWaits in db: %w", err)
	}
	return nil
}

func (d *PGWaitListDao) DeleteByNameAndNamespace(ctx context.Context, name, namespace string) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := cnn.Where("custom_resource->'Metadata'->>'Name' = ? and namespace = ?", name, namespace).Delete(&CustomResourceWaitEntity{}).Error; err != nil {
			return utils.LogError(log, ctx, "error delete CustomResourceWaitEntities by name '%s' and namespace: '%s', error: %w", name, namespace, err)
		}
		return nil
	}); err != nil {
		return utils.LogError(log, ctx, "error deleting CustomResourceWaits in db: %w", err)
	}
	return nil
}
