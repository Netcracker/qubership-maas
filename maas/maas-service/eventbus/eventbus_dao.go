package eventbus

import (
	"context"
	"fmt"
	"github.com/netcracker/qubership-maas/dao"
	"gorm.io/gorm"
)

type EventBusDao interface {
	CreateNotifyListener(ctx context.Context, channelName string) *NotifyListener
	Notify(ctx context.Context, channelName, data string) error
}

type EventBusDaoImpl struct {
	base dao.BaseDao
}

func NewEventbusDao(base dao.BaseDao) EventBusDao {
	return &EventBusDaoImpl{base: base}
}

func (e *EventBusDaoImpl) CreateNotifyListener(ctx context.Context, channelName string) *NotifyListener {
	return NewNotifyListener(e.base.DSN(), channelName)
}

func (e *EventBusDaoImpl) Notify(ctx context.Context, channelName, data string) error {
	if err := e.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Exec("select pg_notify($1, $2)", channelName, data).Error
	}); err != nil {
		return fmt.Errorf("error send notification to channel: %v, error: %w", channelName, err)
	}

	return nil
}
