package eventbus

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/utils"
	"sync"
	"time"
)

type NotifyListener struct {
	m sync.Mutex

	dsn  string
	name string

	ch  chan string
	log logging.Logger
}

func NewNotifyListener(dsn, channelName string) *NotifyListener {
	return &NotifyListener{
		dsn:  dsn,
		name: channelName,
		log:  logging.GetLogger(fmt.Sprintf("NotifyChannel:%v", channelName)),
	}
}

func (nc *NotifyListener) Start(ctx context.Context) error {
	return nc.synchronized(func() error {
		nc.ch = make(chan string)
		go func() {
			nc.retryConnection(ctx, nc.listenMessages)
		}()

		return nil
	})
}

func (nc *NotifyListener) retryConnection(ctx context.Context, handler func(context.Context, *pgx.Conn) error) error {
	for {
		nc.log.InfoC(ctx, "Create new database connection for notify channel")
		cnn, err := pgx.Connect(ctx, nc.dsn)
		if err != nil {
			nc.log.ErrorC(ctx, "error connecting to database: %s, retry in 5 second...", err.Error())
			if utils.CancelableSleep(ctx, 5*time.Second) {
				continue
			} else {
				// context is cancelled
				return ctx.Err()
			}
		}

		err = handler(ctx, cnn)
		switch {
		case isContextCancelled(ctx.Err()):
			return err
		case err != nil:
			nc.log.ErrorC(ctx, "error waiting data change notification: %s", err.Error())
			cnn.Close(ctx)
		default:
			nc.log.InfoC(ctx, "normal exit")
			return nil
		}
	}
}

func (nc *NotifyListener) listenMessages(ctx context.Context, cnn *pgx.Conn) error {
	for {
		nc.log.InfoC(ctx, "Execute listen for channel: %s", nc.name)
		_, err := cnn.Exec(ctx, fmt.Sprintf("listen %s", nc.name))

		nc.log.DebugC(ctx, "Wait for data update event")
		notification, err := cnn.WaitForNotification(ctx)
		if err == nil {
			nc.log.InfoC(ctx, "Received notification: %+v", notification)
			nc.ch <- notification.Payload
		} else {
			return err
		}
	}
}

func isContextCancelled(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func (nc *NotifyListener) Events() <-chan string {
	return nc.ch
}

func (nc *NotifyListener) synchronized(f func() error) error {
	nc.m.Lock()
	defer nc.m.Unlock()

	return f()
}
