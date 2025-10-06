package broadcast

import (
	"context"
	"github.com/stretchr/testify/assert"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestBroadcastSync(t *testing.T) {
	ctx := context.Background()

	b := New[string]()

	execCounter := atomic.Uint32{}
	cb := func(ctx context.Context, data string) {
		execCounter.Add(1)
	}
	id1 := b.AddListener(cb)
	id2 := b.AddListener(cb)

	b.ExecuteListeners(ctx, "one")

	assert.Equal(t, uint32(2), execCounter.Load())

	b.RemoveListener(id1)
	b.ExecuteListeners(ctx, "two")
	assert.Equal(t, uint32(3), execCounter.Load())

	b.RemoveListener(id2)
	b.ExecuteListeners(ctx, "three")
	assert.Equal(t, uint32(3), execCounter.Load())
}

func TestBroadcastAsync(t *testing.T) {
	b := New[string]()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.StartAsyncListenersExecutor(ctx)

	wg := sync.WaitGroup{}
	cb := func(ctx context.Context, data string) {
		wg.Done()
	}
	b.AddListener(cb)
	b.AddListener(cb)

	// start test
	wg.Add(2)
	b.ExecuteListeners(ctx, "one")

	// also test async mode
	wg.Add(2)
	b.ExecuteListenersAsync(ctx, "one")

	// there is no need for any assertion
	// panicked if wait will wait indefinitely by go itself
	wg.Wait()
}

func TestIncorrectInitialization1(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	b := New[string]()
	b.ExecuteListenersAsync(context.Background(), "one")
}

func TestIncorrectInitialization2(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	b := New[string]()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	b.StartAsyncListenersExecutor(ctx)
	b.StartAsyncListenersExecutor(ctx) // panic
}
