package eventbus

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/utils"
	"maas/maas-service/utils/broadcast"
	"sync"
)

type Event struct {
	ClientId uuid.UUID `json:"clientId"`
	Kind     string    `json:"kind"`
	Message  string    `json:"message"`
	Bubble   bool      `json:"bubble"`
}

type Listener func(context.Context, Event)

type eventBus struct {
	m sync.RWMutex

	db          EventBusDao
	channelName string
	clientId    uuid.UUID
	broadcast   *broadcast.Broadcast[Event]
	closeFunc   func()

	log logging.Logger
}

func NewEventBus(db EventBusDao) *eventBus {
	clientId := uuid.New()
	return &eventBus{
		db:          db,
		clientId:    clientId,
		channelName: "eventbus",
		broadcast:   broadcast.New[Event](),
		log:         logging.GetLogger("eventbus:" + clientId.String()),
	}
}

func (eb *eventBus) Start(ctx context.Context) error {
	eb.m.Lock()
	defer eb.m.Unlock()

	eb.log.Info("Start eventbus...")

	notifyListener := eb.db.CreateNotifyListener(ctx, eb.channelName)
	err := notifyListener.Start(ctx)
	if err != nil {
		return fmt.Errorf("error start eventbus: %w", err)
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		wg.Done()

		var event Event
		for {
			select {
			case ev := <-notifyListener.Events():
				err = json.Unmarshal([]byte(ev), &event)
				if err == nil {
					if event.Bubble || event.ClientId != eb.clientId { // skip events from myself if bubble == false
						eb.broadcast.ExecuteListeners(ctx, event)
					}
				} else {
					eb.log.ErrorC(ctx, "Error unmarshal raw data to message: %+v, error: %s", ev, err)
				}
			case <-ctx.Done():
				eb.log.DebugC(ctx, "Context is cancelled, exit listener loop")
				return
			}
		}
	}()

	wg.Wait() // wait till goroutine has been started
	return nil
}

func (eb *eventBus) Send(ctx context.Context, kind string, message string) error {
	return eb.sendEvent(ctx, &Event{ClientId: eb.clientId, Kind: kind, Message: message, Bubble: false})
}

func (eb *eventBus) Broadcast(ctx context.Context, kind string, message string) error {
	return eb.sendEvent(ctx, &Event{ClientId: eb.clientId, Kind: kind, Message: message, Bubble: true})
}

func (eb *eventBus) sendEvent(ctx context.Context, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return utils.LogError(eb.log, ctx, "error serialize event %+v, %w", event, err)
	}
	return eb.db.Notify(ctx, eb.channelName, string(data))
}

func (eb *eventBus) AddListener(listener Listener) {
	eb.broadcast.AddListener(listener)
}
