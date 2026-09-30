package instance

import (
	"context"
	"errors"

	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
)

// defaultInstance reads the instance marked as default. Kafka and RabbitMQ differ only in the
// row they read and the name they log.
func defaultInstance[T any](ctx context.Context, base dao.BaseDao, kind string) (*T, error) {
	data := new(T)
	err := base.WithTx(ctx, func(_ context.Context, cnn *gorm.DB) error {
		return cnn.Where("is_default=true").Take(data).Error
	})

	switch {
	case errors.Is(err, dao.RecordNotFoundInCache):
		return nil, utils.LogError(log, ctx, "default %s instance is not readable: %w", kind, dao.MasterDatabaseUnavailable)
	case errors.Is(err, gorm.ErrRecordNotFound):
		log.WarnC(ctx, "no %s instance registered yet", kind)
		return nil, nil
	case err != nil:
		return nil, utils.LogError(log, ctx, "unknown database error: %w", err)
	default:
		return data, nil
	}
}
