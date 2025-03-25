package dao

import (
	"context"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/testharness"
	"maas/maas-service/utils"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		var counter int32 = 0
		wg := sync.WaitGroup{}
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func() {
				baseDao.WithLock(ctx, "blabla", func(ctx context.Context) error {
					if atomic.AddInt32(&counter, 1) != 1 {
						assert.Fail(t, "Overlapping")
					}

					// increase overlap possibilities
					utils.CancelableSleep(ctx, 100*time.Millisecond)

					if atomic.AddInt32(&counter, -1) != 0 {
						assert.Fail(t, "Overlapping")
					}

					return nil
				})
				wg.Done()
			}()
		}

		wg.Wait()
	})
}

func TestLock_ParallelExecutionOnDifferentId(t *testing.T) {
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		sem := testharness.NewSemaphore(t)

		go baseDao.WithLock(context.Background(), "first", func(ctx context.Context) error {
			sem.Notify("first started")
			sem.Await("second finished", 10*time.Second)
			sem.Notify("first finished")
			return nil
		})

		sem.Await("first started", 10*time.Second)
		baseDao.WithLock(context.Background(), "second", func(ctx context.Context) error {
			sem.Notify("second finished")
			return nil
		})

		sem.Await("first finished", 10*time.Second)
	})
}

func TestLock_ParallelNestedUsing(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		baseDao := New(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			DrMode:   dr.Active,
			PoolSize: 5,
		},
		)
		defer baseDao.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		assert.NoError(t,
			baseDao.UsingDb(ctx, func(cnn *gorm.DB) error {
				return cnn.Exec("create table counter (n int)").Error
			}),
		)

		concurrencyLevel := 100
		wg := sync.WaitGroup{}
		for i := 0; i < concurrencyLevel; i++ {
			wg.Add(1)
			ctx := context.Background()
			go func() {
				defer wg.Done()
				assert.NoError(t, baseDao.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
					return baseDao.WithTx(ctx, func(ctx context.Context, conn2 *gorm.DB) error {
						return baseDao.UsingDb(ctx, func(conn3 *gorm.DB) error {
							return conn3.Exec("insert into counter (n) values(1)").Error
						})
					})
				}))
			}()
		}
		wg.Wait()

		var count int
		assert.NoError(t, baseDao.UsingDb(ctx, func(cnn *gorm.DB) error {
			return cnn.Table("counter").Select("count(*)").Find(&count).Error
		}))
		assert.Equal(t, concurrencyLevel, count)
	})
}
