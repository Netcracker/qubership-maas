package utils

import (
	"context"
	"fmt"
	"time"
)

type CountingMutex struct {
	timeout time.Duration
	slots   chan int
}

func NewCountingMutex(count int, acquireTimeout time.Duration) *CountingMutex {
	slots := make(chan int, count)
	for i := 0; i < count; i++ {
		slots <- i
	}

	return &CountingMutex{timeout: acquireTimeout, slots: slots}
}

func (lc CountingMutex) Exec(ctx context.Context, f func() error) error {
	ctx, cancel := context.WithTimeout(ctx, lc.timeout)
	for {
		select {
		case slot := <-lc.slots:
			defer func() { lc.slots <- slot }() // return slot to pool
			cancel()                            // cancel timeout, because we need to exclude callback execution time from counting
			return f()
		case <-ctx.Done():
			cancel()
			return fmt.Errorf("timeout avaiting lock: %w", ctx.Err())
		}
	}
}
