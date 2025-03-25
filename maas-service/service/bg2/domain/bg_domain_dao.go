package domain

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"maas/maas-service/dao"
	"maas/maas-service/msg"
	"maas/maas-service/utils"
	"time"
)

type BGDomainEntity struct {
	ID          int64
	Controller  sql.NullString `gorm:"column:controller_namespace"`
	Origin      string         `gorm:"column:origin_namespace"`
	Peer        string         `gorm:"column:peer_namespace"`
	CreatedWhen time.Time      `gorm:"default:now()"`
}

func (BGDomainEntity) TableName() string {
	return "bg_domains"
}

type BGStateEntity struct {
	ID         int64
	BgDomainId int64
	OriginNs   BGNamespace `gorm:"serializer:json"`
	PeerNs     BGNamespace `gorm:"serializer:json"`
	UpdateTime time.Time
}

func (BGStateEntity) TableName() string {
	return "bg_states"
}

func (e BGStateEntity) checkEqual(secondBgState BGStateEntity) bool {
	equal := true

	equal = (e.OriginNs.State == secondBgState.OriginNs.State) && equal
	equal = (e.OriginNs.Name == secondBgState.OriginNs.Name) && equal
	equal = (e.OriginNs.Version == secondBgState.OriginNs.Version) && equal

	equal = (e.PeerNs.State == secondBgState.PeerNs.State) && equal
	equal = (e.PeerNs.Name == secondBgState.PeerNs.Name) && equal
	equal = (e.PeerNs.Version == secondBgState.PeerNs.Version) && equal

	return equal
}

func (e BGDomainEntity) toValue() *BGNamespaces {
	return &BGNamespaces{Origin: e.Origin, Peer: e.Peer, ControllerNamespace: e.Controller.String}
}

type BGDomainDaoImpl struct {
	base dao.BaseDao
}

func NewBGDomainDao(base dao.BaseDao) *BGDomainDaoImpl {
	return &BGDomainDaoImpl{base: base}
}

func (bgd *BGDomainDaoImpl) List(ctx context.Context) (*[]BGDomainEntity, error) {
	list := make([]BGDomainEntity, 0)
	return &list, bgd.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Find(&list).Error
	})
}

func (bgd *BGDomainDaoImpl) Bind(ctx context.Context, left string, right string, controller string) error {
	return bgd.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {

		bgEntity, err := bgd.FindByNamespace(ctx, left)
		if err != nil {
			return utils.LogError(log, ctx, "error during FindByNamespace while in Bind: %w", err)
		}

		if bgEntity != nil && bgEntity.Controller.String == "" {
			bgEntity.Controller = sql.NullString{
				String: controller,
				Valid:  controller != "",
			}
			return cnn.Save(bgEntity).Error
		}

		return cnn.Create(&BGDomainEntity{Origin: left, Peer: right,
			Controller: sql.NullString{
				String: controller,
				Valid:  controller != "",
			},
		}).Error
	})
}

func (bgd *BGDomainDaoImpl) FindByNamespace(ctx context.Context, namespace string) (*BGDomainEntity, error) {
	domain := BGDomainEntity{}
	err := bgd.base.UsingDb(ctx, func(conn *gorm.DB) error {
		return conn.Where(`? in ("origin_namespace", "peer_namespace", "controller_namespace")`, namespace).First(&domain).Error
	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return &domain, err
	}
}

// Unbind remove any namespace included in domain, will destroy whole domain
func (bgd *BGDomainDaoImpl) Unbind(ctx context.Context, namespace string) (*BGDomainEntity, error) {
	domain := BGDomainEntity{}
	return &domain, bgd.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		res := cnn.Clauses(clause.Returning{}).
			Where(`lower(?) in (lower("origin_namespace"), lower("peer_namespace"))`, namespace).
			Delete(&domain)

		if res.RowsAffected == 0 {
			return fmt.Errorf("namespace is not yet bound: %v", namespace)
		} else {
			return res.Error
		}
	})
}

func (bgd *BGDomainDaoImpl) UpdateController(ctx context.Context, origin, controller string) error {
	return bgd.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if domain, err := bgd.FindByNamespace(ctx, origin); err != nil {
			return fmt.Errorf("error getting domain: %w", err)
		} else if domain == nil {
			return fmt.Errorf("can't update controller namespace for non existing domain: %v: %w", origin, msg.BadRequest)
		} else {
			if domain.Controller.String == "" {
				// need migration
				res := cnn.Model(domain).Where(`lower(?) in (lower("origin_namespace"), lower("peer_namespace"))`, origin).UpdateColumn("controller_namespace", controller)
				if res.Error != nil {
					return fmt.Errorf("unexpected error update controller namespace value for domain: %w", res.Error)
				}
				if res.RowsAffected != 1 {
					return fmt.Errorf("unexpected error update controller namespace value for domain: no rows updated")
				}
				return nil
			} else if domain.Controller.String == controller {
				// already updated in expected value
				return nil
			} else {
				return fmt.Errorf("controller namespace already set and it differs from expected: %v, expected: %v : %w", domain.Controller.String, controller, msg.BadRequest)
			}
		}
	})
}

func (bgd *BGDomainDaoImpl) UpdateState(ctx context.Context, bgStateEntity BGStateEntity) error {
	return bgd.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		if err := cnn.Create(&bgStateEntity).Error; err != nil {
			return fmt.Errorf("error update domain state: %w", err)
		}

		// housekeeping: leave only 100 recent state change records
		if _, err := bgd.DeleteObsoleteStates(ctx, 100); err != nil {
			return fmt.Errorf("error delete obsolete states from domain state table: %w", err)
		}
		return nil
	})
}

func (bgd *BGDomainDaoImpl) DeleteObsoleteStates(ctx context.Context, offset int64) (int64, error) {
	var count int64 = 0
	return count, bgd.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		res := cnn.Exec(`delete from bg_states where id in (select id from bg_states order by update_time desc offset ?)`, offset)
		count = res.RowsAffected
		return res.Error
	})
}

func (bgd *BGDomainDaoImpl) GetState(ctx context.Context, domainId int64) (*BGStateEntity, error) {
	entity := BGStateEntity{}
	err := bgd.base.UsingDb(ctx, func(conn *gorm.DB) error {
		return conn.Where(`bg_domain_id=?`, domainId).Order("update_time DESC").First(&entity).Error
	})

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return nil, nil
	case err != nil:
		return nil, err
	default:
		return &entity, err
	}
}
