package eventbus

import (
	"context"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dao"
	dbc "maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/testharness"
	"maas/maas-service/utils"
	"testing"
	"time"
)

func TestNotifyChannel(t *testing.T) {
	ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelFunc()

	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		base := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 3,
			DrMode:   dr.Active,
		})
		defer base.Close()
		db := NewEventbusDao(base)

		listener := db.CreateNotifyListener(ctx, "eventbus")
		assert.NoError(t, listener.Start(ctx))

		sem := testharness.NewSemaphore(t)
		go func() {
			for {
				select {
				case data := <-listener.Events():
					sem.Notify(data)
				case <-ctx.Done():
					return
				}
			}
		}()

		utils.CancelableSleep(ctx, 1*time.Second) // give a spin for listener start
		assert.NoError(t, db.Notify(ctx, "eventbus", "hello"))
		sem.Await("hello", 20*time.Second)
		assert.True(t, true)
	})
}

func TestDatabaseFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		localAddr := "localhost:5433"
		proxy := testharness.NewTCPProxy(localAddr, tdb.Addr())
		assert.NoError(t, proxy.Start(ctx))

		nonstableBase := dao.New(&dbc.Config{
			Addr:     localAddr,
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Active,
		})
		defer nonstableBase.Close()

		base := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Active,
		})
		defer nonstableBase.Close()

		notifyDao := NewEventbusDao(base)
		listenDao := NewEventbusDao(nonstableBase)

		listener := listenDao.CreateNotifyListener(ctx, "eventbus")
		assert.NoError(t, listener.Start(ctx))
		go func() {
			for !isContextCancelled(ctx.Err()) {
				notifyDao.Notify(ctx, "eventbus", "data")
				utils.CancelableSleep(ctx, 2*time.Second)
			}
		}()

		// =======================================
		// emulate db connection failure
		// =======================================
		proxy.Close()
		messages := awaitMessages(ctx, listener.Events(), 30*time.Second)
		assert.Equal(t, 0, len(messages))

		// =======================================
		// restore db connection
		// =======================================
		proxy.Start(ctx)
		messages = awaitMessages(ctx, listener.Events(), 10*time.Second)
		assert.True(t, len(messages) > 0)
	})
}

func awaitMessages[T any](ctx context.Context, ch <-chan T, duration time.Duration) []T {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var result []T
	for {
		select {
		case d := <-ch:
			result = append(result, d)
		case <-ctx.Done():
			return result
		}
	}
}
