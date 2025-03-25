package replicator

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"maas/maas-service/testharness"
	"maas/maas-service/utils"
	"testing"
	"time"
)

func TestMasterMonitor(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {

		db, err := gorm.Open(postgres.Open(tdb.DSN()))
		assert.NoError(t, err)

		mm := NewMasterMonitor(db, 100*time.Millisecond)

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		mm.Start(ctx)
		utils.CancelableSleep(ctx, 1*time.Second)

		assert.True(t, mm.IsAlive())
		assert.True(t, mm.Status() == nil)

		tdb.Close(t)
		utils.CancelableSleep(ctx, 1*time.Second)

		assert.False(t, mm.IsAlive())
		assert.NotNil(t, mm.Status())
		fmt.Printf("Status: %+v\n", mm.Status())
	})
}
