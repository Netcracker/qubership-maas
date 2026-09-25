package instance

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/dao/mock_dao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// readDefault is what the two DAOs have in common: the same lookup over a different row.
type readDefault struct {
	name string
	read func(ctx context.Context, base dao.BaseDao) (any, error)
}

func defaultInstanceReaders() []readDefault {
	return []readDefault{
		{"kafka", func(ctx context.Context, base dao.BaseDao) (any, error) {
			return KafkaInstancesDaoImpl{base: base}.GetDefaultInstance(ctx)
		}},
		{"rabbitmq", func(ctx context.Context, base dao.BaseDao) (any, error) {
			return RabbitInstancesDaoImpl{base: base}.GetDefaultInstance(ctx)
		}},
	}
}

func baseDaoAnswering(t *testing.T, answer error) dao.BaseDao {
	base := mock_dao.NewMockBaseDao(gomock.NewController(t))
	base.EXPECT().WithTx(gomock.Any(), gomock.Any()).Return(answer)
	return base
}

// The registration exists in every working installation, so a miss the cache answered means it
// cannot be read. Reported as an empty result it reaches the client as a conflict.
func TestGetDefaultInstance_MissFromCacheIsUnavailable(t *testing.T) {
	cacheMiss := fmt.Errorf("%w: %w", dao.RecordNotFoundInCache, gorm.ErrRecordNotFound)

	for _, reader := range defaultInstanceReaders() {
		t.Run(reader.name, func(t *testing.T) {
			_, err := reader.read(context.Background(), baseDaoAnswering(t, cacheMiss))

			require.Error(t, err)
			assert.ErrorIs(t, err, dao.MasterDatabaseUnavailable)
		})
	}
}

// A miss the master answered is an empty table, which is a legitimate state.
func TestGetDefaultInstance_MissFromMasterIsEmpty(t *testing.T) {
	for _, reader := range defaultInstanceReaders() {
		t.Run(reader.name, func(t *testing.T) {
			_, err := reader.read(context.Background(), baseDaoAnswering(t, gorm.ErrRecordNotFound))

			assert.NoError(t, err)
		})
	}
}

// The row read without error is returned.
func TestGetDefaultInstance_RowIsReturned(t *testing.T) {
	for _, reader := range defaultInstanceReaders() {
		t.Run(reader.name, func(t *testing.T) {
			instance, err := reader.read(context.Background(), baseDaoAnswering(t, nil))

			require.NoError(t, err)
			assert.NotNil(t, instance)
		})
	}
}

// Anything else keeps its own message.
func TestGetDefaultInstance_OtherErrorIsPassedOn(t *testing.T) {
	boom := errors.New("connection reset")

	for _, reader := range defaultInstanceReaders() {
		t.Run(reader.name, func(t *testing.T) {
			_, err := reader.read(context.Background(), baseDaoAnswering(t, boom))

			require.Error(t, err)
			assert.ErrorIs(t, err, boom)
			assert.NotErrorIs(t, err, dao.MasterDatabaseUnavailable)
		})
	}
}
