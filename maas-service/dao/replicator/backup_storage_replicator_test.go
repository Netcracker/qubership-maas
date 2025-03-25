package replicator

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"maas/maas-service/dao/db"
	"maas/maas-service/dao/schema"
	"maas/maas-service/dr"
	"maas/maas-service/testharness"
	"maas/maas-service/utils"
	"testing"
	"time"
)

var logger = logging.GetLogger("cache-sync")

func TestBasicSync(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		ctx, cancel := context.WithCancel(context.Background())
		defer func() {
			logger.InfoC(ctx, ">> stop synchronizer")
			cancel()
		}()

		master, err := gorm.Open(postgres.Open(tdb.DSN()))
		if err != nil {
			assert.FailNow(t, "error open master connection: %s", err)
		}
		assert.NoError(t, master.Exec("create table teams (name varchar(100) not null)").Error)
		assert.NoError(t, master.Exec("create view teams_view as select * from teams").Error)

		schema.Migrate(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Active,
		})

		monitor := NewMasterMonitor(master, 1*time.Second)
		monitor.Start(ctx)
		syncer := NewBackupStorageReplicator(tdb.DSN(), master, monitor)
		// start syncer with cancelable context
		assert.NoError(t, syncer.Start(ctx, 10*time.Minute))

		// perform test
		logger.InfoC(ctx, ">> insert master record")
		assert.NoError(t, master.Exec("insert into teams (name) values('lakers')").Error)
		utils.CancelableSleep(ctx, 1*time.Second)
		logger.InfoC(ctx, ">> check slave record")
		var value string
		assert.NoError(t, syncer.DB().Table("teams").Take(&value).Error)
		assert.Equal(t, value, "lakers")

		logger.InfoC(ctx, ">> update master record")
		assert.NoError(t, master.Exec("update teams set name='bucks'").Error)
		utils.CancelableSleep(ctx, 1*time.Second)
		logger.InfoC(ctx, ">> check slave record")
		assert.NoError(t, syncer.DB().Table("teams").Take(&value).Error)
		assert.Equal(t, value, "bucks")

		logger.InfoC(ctx, ">> delete master record")
		assert.NoError(t, master.Exec("delete from teams").Error)
		utils.CancelableSleep(ctx, 1*time.Second)
		logger.InfoC(ctx, ">> check slave record")
		assert.ErrorIs(t, syncer.DB().Table("teams").Take(&value).Error, gorm.ErrRecordNotFound)
	})
}
