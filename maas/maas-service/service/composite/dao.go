package composite

import (
	"context"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("composite")
}

type registrationEntity struct {
	Id        int64  `gorm:"primaryKey"`
	Baseline  string `gorm:"column:base_namespace"`
	Namespace string
}

func (_ registrationEntity) TableName() string {
	return "composite_namespaces_v2"
}

type PGRegistrationDao struct {
	base dao.BaseDao
}

func NewPGRegistrationDao(dao dao.BaseDao) *PGRegistrationDao {
	return &PGRegistrationDao{base: dao}
}

func (d *PGRegistrationDao) Upsert(ctx context.Context, registration *CompositeRegistration) error {
	log.InfoC(ctx, "Upsert composite registration: %+v", registration)
	return d.base.WithLock(ctx, registration.Id, func(ctx context.Context) error {
		return d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
			if registration.ModifyIndex != nil {
				currentModifyIndex, err := getCurrentModifyIndex(ctx, cnn, registration.Id)
				if err != nil {
					return err
				}
				if *registration.ModifyIndex < currentModifyIndex {
					return utils.LogError(log, ctx, "new modify index '%d' cannot be less than the current index '%d': %w", *registration.ModifyIndex, currentModifyIndex, msg.BadRequest)
				}
			}

			if err := d.DeleteByBaseline(ctx, registration.Id); err != nil {
				return utils.LogError(log, ctx, "error update composite namespace: %w", err)
			}

			insertions := make([]registrationEntity, len(registration.Namespaces))
			for i, namespace := range registration.Namespaces {
				insertions[i] = registrationEntity{Baseline: registration.Id, Namespace: namespace}
			}

			if err := cnn.Create(insertions).Error; err != nil {
				if pgErr, ok := err.(*pgconn.PgError); ok {
					return utils.LogError(log, ctx, "error insert composite registration %+v, %s: %w", registration, pgErr.Detail, pgErr)
				}

				// generic message
				return utils.LogError(log, ctx, "error insert composite registration %+v: %w", registration, err)
			}

			if registration.ModifyIndex != nil {
				err := createCurrentModifyIndex(ctx, cnn, registration.Id, registration.ModifyIndex)

				if err != nil {
					return err
				}
			}

			return nil
		})
	})
}

func (d *PGRegistrationDao) GetByBaseline(ctx context.Context, baseline string) (*CompositeRegistration, error) {
	entries := make([]registrationEntity, 0)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Order("namespace").Where("base_namespace = ?", baseline).Find(&entries).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error finding composite registration by baseline '%s': %w", baseline, err)
	}

	if len(entries) == 0 {
		return nil, nil
	}

	return reformat(&entries), nil
}

func reformat(entries *[]registrationEntity) *CompositeRegistration {
	// reformat to normal form
	result := CompositeRegistration{Id: (*entries)[0].Baseline}
	for _, row := range *entries {
		result.Namespaces = append(result.Namespaces, row.Namespace)
	}
	return &result
}

func (d *PGRegistrationDao) DeleteByBaseline(ctx context.Context, baseline string) error {
	if err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		return cnn.Where("base_namespace = ?", baseline).Delete(&registrationEntity{}).Error
	}); err != nil {
		return utils.LogError(log, ctx, "error delete RegistrationEntity by baseline: %s, error: %w", baseline, err)
	}
	return nil
}

func (d *PGRegistrationDao) GetByNamespace(ctx context.Context, namespace string) (*CompositeRegistration, error) {
	var res *[]registrationEntity
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		result := cnn.Order("namespace").Where("base_namespace=(?)",
			cnn.Select("base_namespace").Where("namespace = ?", namespace).Model(&res),
		).Find(&res)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			res = nil
		}
		return nil
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error getting RegistrationEntity by namespace '%s' in db: %w", namespace, err)
	}

	if res == nil {
		return nil, nil
	} else {
		return reformat(res), nil
	}
}

func (d *PGRegistrationDao) List(ctx context.Context) ([]CompositeRegistration, error) {
	res := make([]registrationEntity, 0)
	if err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Order("base_namespace, namespace").Find(&res).Error
	}); err != nil {
		return nil, utils.LogError(log, ctx, "error finding all RegistrationEntities: %w", err)
	}

	// this loop assumes that query results is ordered
	registrations := make([]CompositeRegistration, 0)
	for _, row := range res {
		if len(registrations) == 0 {
			registrations = append(registrations, CompositeRegistration{Id: row.Baseline, Namespaces: []string{row.Namespace}})
		} else {
			last := &registrations[len(registrations)-1]
			if last.Id == row.Baseline {
				last.Namespaces = append(last.Namespaces, row.Namespace)
			} else {
				registrations = append(registrations, CompositeRegistration{Id: row.Baseline, Namespaces: []string{row.Namespace}})
			}
		}
	}

	return registrations, nil
}

func getCurrentModifyIndex(ctx context.Context, cnn *gorm.DB, compositeId string) (uint64, error) {
	var currentModifyIndex uint64
	result := cnn.Raw(`
						SELECT c.modify_index
						FROM composite_namespace_modify_indexes c
						JOIN composite_namespaces_v2 n ON n.id = c.composite_namespace_id
						WHERE n.base_namespace = ?
			`, compositeId).Scan(&currentModifyIndex)
	if result.Error != nil {
		return 0, utils.LogError(log, ctx, "error get current modify index: %w", result.Error)
	}

	return currentModifyIndex, nil
}

func createCurrentModifyIndex(ctx context.Context, cnn *gorm.DB, compositeId string, modifyIndex *uint64) error {
	result := cnn.Exec(`
				WITH ns AS (
					SELECT id
					FROM composite_namespaces_v2
					WHERE base_namespace = $1 AND namespace = $1
				)
				INSERT INTO composite_namespace_modify_indexes (composite_namespace_id, modify_index)
				SELECT id, $2 FROM ns
			`, compositeId, modifyIndex)

	if result.Error != nil {
		return utils.LogError(log, ctx, "error insert composite registration modify index compositeId='%s', modifyIndex='%s': %w", compositeId, modifyIndex, result.Error)
	}
	return nil
}
