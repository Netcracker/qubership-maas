package schema

import (
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/testharness"
	"testing"
)

func TestMigrateActive(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		Migrate(&db.Config{
			Addr:      tdb.Addr(),
			User:      tdb.Username(),
			Password:  tdb.Password(),
			Database:  tdb.DBName(),
			DrMode:    dr.Active,
			CipherKey: configloader.GetKoanf().MustString("db.cipher.key"),
		})

		// test successful migration for dr
		Migrate(&db.Config{
			Addr:      tdb.Addr(),
			User:      tdb.Username(),
			Password:  tdb.Password(),
			Database:  tdb.DBName(),
			DrMode:    dr.Disabled,
			CipherKey: configloader.GetKoanf().MustString("db.cipher.key"),
		})
	})
}
