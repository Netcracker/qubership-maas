package utils

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCountingMutex(t *testing.T) {
	cm := NewCountingMutex(2, 10*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	wg := sync.WaitGroup{}
	contr := atomic.Int32{}
	for i := 0; i < 30; i++ {
		wg.Add(1)

		go cm.Exec(ctx, func() error {
			defer func() {
				contr.Add(-1)
				wg.Done()
			}()

			contr.Add(1)

			CancelableSleep(ctx, 10*time.Millisecond)
			assert.True(t, contr.Load() <= 2)
			CancelableSleep(ctx, 10*time.Millisecond)

			return nil
		})
	}

	wg.Wait()
}

func TestNewCountingMutexTimeout(t *testing.T) {
	ctx := context.Background()
	cm := NewCountingMutex(0, 100*time.Millisecond)
	assert.ErrorIs(t, cm.Exec(ctx, nil), context.DeadlineExceeded)
}
