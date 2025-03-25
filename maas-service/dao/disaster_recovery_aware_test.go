package dao

import (
	"context"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/model"
	"maas/maas-service/testharness"
	"testing"
)

func TestDRErrorTranslation(t *testing.T) {
	ctx := context.Background()
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		// this dao only need to run migrations
		baseDao := New(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 1,
			DrMode:   dr.Active,
		})
		baseDao.Close()

		readOnlyDao := New(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 1,
			DrMode:   dr.Disabled,
		})
		defer readOnlyDao.Close()

		err := readOnlyDao.UsingDb(ctx, func(cnn *gorm.DB) error {
			account := &model.Account{
				Username:         "scott",
				Roles:            []model.RoleName{"agent"},
				Salt:             "abc",
				Password:         "cde",
				Namespace:        "core-dev",
				DomainNamespaces: nil,
			}
			return cnn.Create(account).Error
		})

		assert.ErrorIs(t, err, DatabaseIsNotActiveError)
	})
}
