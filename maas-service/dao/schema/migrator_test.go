package schema

import (
	"github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/testharness"
	"testing"
)

func TestMigrateActive(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		Migrate(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Active,
		})

		// test successful migration for dr
		Migrate(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Disabled,
		})
	})
}
