package eventbus

import (
	"context"
	"github.com/stretchr/testify/assert"
	"log"
	"maas/maas-service/dao"
	"maas/maas-service/testharness"
	"maas/maas-service/utils"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

func TestEventBusAPI(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelFunc()

		sem := testharness.NewSemaphore(t)

		broadcaster1 := NewEventBus(NewEventbusDao(baseDao))

		broadcaster1.AddListener(func(ctx context.Context, event Event) {
			if event.Kind == "all" {
				sem.Notify(event.Kind + ":" + event.Message)
				return
			}

			// sender shouldn't receive its own messages
			assert.Fail(t, "received its own message")
		})
		assert.NoError(t, broadcaster1.Start(ctx))

		broadcaster2 := NewEventBus(NewEventbusDao(baseDao))

		broadcaster2.AddListener(func(ctx context.Context, event Event) {
			sem.Notify(event.Kind + ":" + event.Message)
		})
		assert.NoError(t, broadcaster2.Start(ctx))
		// give a spin for a listeners to start
		utils.CancelableSleep(ctx, 1*time.Second)

		assert.NoError(t, broadcaster1.Send(ctx, "siblings", "data1"))

		sem.Await("siblings:data1", 10*time.Second)

		// test bubble message
		assert.NoError(t, broadcaster2.Broadcast(ctx, "all", "data2"))
		sem.Await("all:data2", 10*time.Second)
		sem.Await("all:data2", 10*time.Second)
	})
}

func TestDeliveryGuarantee(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelFunc := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelFunc()

		broadcaster := NewEventBus(NewEventbusDao(baseDao))

		messagesCount := 100
		counter := &atomic.Uint32{}
		counter.Store(0)
		sem := testharness.NewSemaphore(t)
		broadcaster.AddListener(func(ctx context.Context, event Event) {
			if n, err := strconv.Atoi(event.Message); err == nil {
				prev := counter.Load()
				delta := n - int(prev)
				if delta != 1 {
					assert.Fail(t, "continuity violation prev=%d, received=%d, delta=%d", prev, n, delta)
				}
				counter.Store(uint32(n))

				if n == messagesCount {
					sem.Notify("finished")
				}
			} else {
				log.Panicf("conversion error: %v", event.Message)
			}
		})
		assert.NoError(t, broadcaster.Start(ctx))
		// give a spin for a listeners to start
		utils.CancelableSleep(ctx, 1*time.Second)

		for i := 1; i <= messagesCount; i++ {
			broadcaster.Broadcast(ctx, "newermind", strconv.Itoa(i))
		}
		sem.Await("finished", 1*time.Minute)
	})
}
