package dao

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"gorm.io/gorm"
	"sync/atomic"
)

type TransactionContext struct {
	// on begin transaction Gorm creates object with allocated
	// connection and opened transaction represented as DB interface
	txCnn   *gorm.DB
	txNum   int64
	nesting int
}

const TransactionContextKey = "tx"

var txCounter int64
var txLog = logging.GetLogger("tx")

func newTransactionContext(ctx context.Context) (context.Context, *TransactionContext) {
	tx := &TransactionContext{txNum: atomic.AddInt64(&txCounter, 1), nesting: 1}
	return context.WithValue(ctx, TransactionContextKey, tx), tx
}

func WithTransactionContextValue[T any](ctx context.Context, f func(ctx context.Context) (T, error)) (T, error) {
	var tx *TransactionContext
	if tx = TransactionContextOf(ctx); tx == nil {
		ctx, tx = newTransactionContext(ctx)
		txLog.InfoC(ctx, "Starting transaction: %+v", tx)
	} else {
		// nested transactions is not supported, use already opened
		tx.nesting += 1
		txLog.InfoC(ctx, "Use already opened global transaction: %+v", tx)
	}

	result, err := f(ctx)
	if tx.nesting > 1 {
		txLog.InfoC(ctx, "Ignore committing nested transaction context: %+v", tx)
		tx.nesting -= 1
		return result, err
	}

	if err == nil {
		txLog.InfoC(ctx, "Commit transaction context: %+v", tx)
		return result, tx.commit()
	} else {
		txLog.InfoC(ctx, "Rollback transaction context: %+v", tx)
		errRollback := tx.rollback()
		if errRollback != nil {
			return result, fmt.Errorf("error rolling back transaction: %v, original error: %w", errRollback, err)
		}
		return result, err
	}
}

func WithTransactionContext(ctx context.Context, f func(ctx context.Context) error) error {
	_, err := WithTransactionContextValue(ctx, func(ctx context.Context) (interface{}, error) {
		return nil, f(ctx)
	})
	return err

}
func TransactionContextOf(ctx context.Context) *TransactionContext {
	if ctx.Value(TransactionContextKey) != nil {
		return ctx.Value(TransactionContextKey).(*TransactionContext)
	} else {
		return nil
	}
}

func (tx *TransactionContext) Begin(cnn *gorm.DB) *gorm.DB {
	if tx.txCnn == nil {
		tx.txCnn = cnn.Begin()
	}

	return tx.txCnn
}

func (tx *TransactionContext) commit() error {
	if tx.txCnn == nil { // tx not yet started or already closed
		return nil
	}

	err := tx.txCnn.Commit().Error
	tx.txCnn = nil
	return err
}
func (tx *TransactionContext) rollback() error {
	if tx.txCnn == nil { // tx not yet started or already closed
		return nil
	}

	err := tx.txCnn.Rollback().Error
	tx.txCnn = nil
	return err
}

func (tx *TransactionContext) String() string {
	return fmt.Sprintf("tx#%d:%d", tx.txNum, tx.nesting)
}

func (tx *TransactionContext) TxCnn() *gorm.DB {
	return tx.txCnn
}
