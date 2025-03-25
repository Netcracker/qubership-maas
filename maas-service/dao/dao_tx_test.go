package dao

import (
	"context"
	"errors"
	"fmt"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"maas/maas-service/testharness"
	"testing"
	"time"
)

func TestAtomicTransactions(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		lock := &Lock{"TestAtomicTransactions"}
		baseDao.UsingDb(ctx, func(db *gorm.DB) error {
			return db.Create(lock).Error
		})
		defer baseDao.db().Delete(lock)

		// test record
		var exists bool
		assert.NoError(t, baseDao.db().Model(Lock{}).
			Select("count(*) > 0").
			Where("id = ?", lock.Id).
			Find(&exists).
			Error)
		assert.True(t, exists, "Record not found")
	})
}

func TestLocalTransactionsRollback(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		lock := &Lock{"TestLocalTransactionsRollback"}
		baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
			if db.Create(lock).Error != nil {
				panic("Unexpected error")
			}

			return errors.New("emulate error that should lead to tx rollback")
		})
		defer baseDao.db().Delete(lock)

		// test record
		var exists bool
		assert.NoError(t, baseDao.db().Model(Lock{}).
			Select("count(*) > 0").
			Where("id = ?", lock.Id).
			Find(&exists).
			Error)
		assert.False(t, exists, "Transaction should be rolled back")
	})
}

func TestLocalTransactions(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		lock := &Lock{"TestLocalTransactionsRollback"}
		baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
			return db.Create(lock).Error
		})
		defer baseDao.db().Delete(lock)

		// test record
		var exists bool
		assert.NoError(t, baseDao.db().Model(Lock{}).
			Select("count(*) > 0").
			Where("id = ?", lock.Id).
			Find(&exists).
			Error)
		assert.True(t, exists, "Record not saved")
	})
}

func TestGlobalTransactions(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		sem := testharness.NewSemaphore(t)

		// use lock table just for its structure simplicity, not for intended purpose
		lock := &Lock{"TestGlobalTransactions"}
		go func() {
			WithTransactionContext(ctx, func(ctx context.Context) error {
				fmt.Printf("Insert test record\n")
				baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
					return db.Create(lock).Error
				})

				baseDao.UsingDb(ctx, func(db *gorm.DB) error {
					var created bool
					assert.NoError(t, db.Model(Lock{}).
						Select("count(*) > 0").
						Where("id = ?", lock.Id).
						Find(&created).
						Error)
					assert.True(t, created)
					return nil
				})

				sem.Notify("check")                   // unlock check visibility code block
				sem.Await("continue", 10*time.Second) // wait answer, that checks has been done

				fmt.Printf("Delete test record\n")
				baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
					return db.Delete(lock).Error
				})
				fmt.Printf("Finished\n")
				return nil
			})

			sem.Notify("finished")
		}()

		sem.Await("check", 10*time.Second)
		fmt.Printf("Check changes visibility: search for record\n")

		var exists bool
		assert.NoError(t, baseDao.db().Model(Lock{}).
			Select("count(*) > 0").
			Where("id = ?", lock.Id).
			Find(&exists).
			Error)
		assert.False(t, exists, "Transaction isolation error!")
		sem.Notify("continue")

		sem.Await("finished", 10*time.Second)
	})
}

func TestGlobalTransactions_RollbackWithInnerTransaction(t *testing.T) {
	ctx := context.Background()
	WithSharedDao(t, func(baseDao *BaseDaoImpl) {
		// prepare structure
		err := baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
			return db.Exec("create table tx_test (name varchar)").Error
		})
		assert.NoError(t, err)

		err = WithTransactionContext(ctx, func(ctx context.Context) error {
			err = baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
				res := db.Exec("insert into tx_test values('1')")
				assert.NoError(t, res.Error)
				assert.Equal(t, 1, int(res.RowsAffected))
				return nil
			})
			assert.NoError(t, err)

			//nested transaction
			err = WithTransactionContext(ctx, func(ctx context.Context) error {
				return baseDao.WithTx(ctx, func(ctx context.Context, db *gorm.DB) error {
					res := db.Exec("insert into tx_test values('1')")
					assert.NoError(t, res.Error)
					assert.Equal(t, 1, int(res.RowsAffected))
					return nil
				})
			})
			assert.NoError(t, err)

			return fmt.Errorf("some error that should cause rollback all changes")
		})
		assert.Error(t, err)

		// all inserted records should be rolled back
		var count int64
		err = baseDao.UsingDb(ctx, func(db *gorm.DB) error {
			return db.Table("tx_test").Select("name").Count(&count).Error
		})

		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}
