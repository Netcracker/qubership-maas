package broadcast

import (
	"context"
	"sync"
	"sync/atomic"
)

type ListenerId uint64
type asyncEvent[T any] struct {
	ctx  context.Context
	data T
}

type Broadcast[T any] struct {
	counter     *atomic.Uint64
	listeners   *sync.Map
	eventsQueue chan asyncEvent[T]
}

func New[T any]() *Broadcast[T] {
	return &Broadcast[T]{counter: &atomic.Uint64{}, listeners: &sync.Map{}}
}

func (b *Broadcast[T]) StartAsyncListenersExecutor(ctx context.Context) {
	if b.eventsQueue != nil {
		panic("Broadcast.StartAsyncListener already started")
	}

	b.eventsQueue = make(chan asyncEvent[T])
	go func() {
		defer func() {
			close(b.eventsQueue)
			b.eventsQueue = nil
		}()

		for {
			select {
			case <-ctx.Done():
				return
			case event := <-b.eventsQueue:
				b.ExecuteListeners(event.ctx, event.data)
			}
		}
	}()
}
func (b *Broadcast[T]) AddListener(callback func(ctx context.Context, data T)) ListenerId {
	id := ListenerId(b.counter.Add(1))
	b.listeners.Store(id, callback)
	return id
}

func (b *Broadcast[T]) RemoveListener(id ListenerId) {
	b.listeners.Delete(id)
}

func (b *Broadcast[T]) ExecuteListeners(ctx context.Context, data T) {
	b.listeners.Range(func(_ any, value any) bool {
		callback := value.(func(context.Context, T))
		callback(ctx, data)
		return true
	})
}

func (b *Broadcast[T]) ExecuteListenersAsync(ctx context.Context, data T) {
	if b.eventsQueue == nil {
		panic("Async executor not yet started. Call Broadcast.StartAsyncListener first")
	}

	b.eventsQueue <- asyncEvent[T]{ctx, data}
}
