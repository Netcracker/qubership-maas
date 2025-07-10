package pg

import (
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"testing"
)

func TestDialectorWithCallbacks(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		db, err := gorm.Open(Open(tdb.DSN(), dr.Active))
		assert.NoError(t, err)

		err = db.Exec("create table test (val varchar(255))").Error
		assert.NoError(t, err)

		err = db.Exec("insert into test (val) values ('42')").Error
		assert.NoError(t, err)

		db, err = gorm.Open(Open(tdb.DSN(), dr.Standby))
		assert.NoError(t, err)

		err = db.Exec("insert into test (val) values ('42')").Error
		var pgError *pgconn.PgError
		assert.ErrorAs(t, err, &pgError)
		assert.Equal(t, "25006", pgError.Code)
	})
}
