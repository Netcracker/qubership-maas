package dao

import (
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/testharness"
	"testing"
)

func WithSharedDao(t *testing.T, testRunnable func(impl *BaseDaoImpl)) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		baseDao := New(&db.Config{
			Addr:      tdb.Addr(),
			User:      tdb.Username(),
			Password:  tdb.Password(),
			Database:  tdb.DBName(),
			PoolSize:  10,
			DrMode:    dr.Active,
			CipherKey: configloader.GetKoanf().MustString("db.cipher.key"),
		})
		defer baseDao.Close()

		testRunnable(baseDao)
	})
}
