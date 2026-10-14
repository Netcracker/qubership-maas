package instance

import (
	"context"
	"testing"

	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestRabbitInstanceUrlsUnique(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		rabbitDao := NewRabbitInstancesDao(baseDao)

		_, err := rabbitDao.InsertInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "first", ApiUrl: "http://rabbit-1:15672/api", AmqpUrl: "amqp://rabbit-1:5672", User: "u", Password: "p",
		})
		assert.NoError(t, err)

		_, err = rabbitDao.InsertInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "second", ApiUrl: "http://rabbit-1:15672/api", AmqpUrl: "amqp://rabbit-2:5672", User: "u", Password: "p",
		})
		assert.ErrorIs(t, err, msg.Conflict)
		assert.ErrorContains(t, err, "apiUrl `http://rabbit-1:15672/api' is already used")

		_, err = rabbitDao.InsertInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "second", ApiUrl: "http://rabbit-2:15672/api", AmqpUrl: "amqp://rabbit-1:5672", User: "u", Password: "p",
		})
		assert.ErrorIs(t, err, msg.Conflict)
		assert.ErrorContains(t, err, "amqpUrl `amqp://rabbit-1:5672' is already used")

		second := &model.RabbitInstance{
			Id: "second", ApiUrl: "http://rabbit-2:15672/api", AmqpUrl: "amqp://rabbit-2:5672", User: "u", Password: "p",
		}
		_, err = rabbitDao.InsertInstanceRegistration(ctx, second)
		assert.NoError(t, err)

		_, err = rabbitDao.UpdateInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "second", ApiUrl: "http://rabbit-1:15672/api", AmqpUrl: "amqp://rabbit-2:5672", User: "u", Password: "p",
		})
		assert.ErrorIs(t, err, msg.Conflict)
		assert.ErrorContains(t, err, "apiUrl `http://rabbit-1:15672/api' is already used")

		_, err = rabbitDao.InsertInstanceRegistration(ctx, second)
		assert.ErrorIs(t, err, msg.Conflict)
		assert.ErrorContains(t, err, "Instance with id=second already registered")
	})
}

func TestRabbitInstanceUpdateKeepsOwnership(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		rabbitDao := NewRabbitInstancesDao(baseDao)

		inserted, err := rabbitDao.InsertInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "owned", ApiUrl: "http://rabbit:15672/api", AmqpUrl: "amqp://rabbit:5672", User: "u", Password: "p",
		})
		assert.NoError(t, err)
		assert.False(t, inserted.ManagedByOperator)

		stored, err := rabbitDao.GetInstanceById(ctx, "owned")
		assert.NoError(t, err)
		assert.Equal(t, model.InstanceOwnership{}, stored.InstanceOwnership)

		ownership := model.InstanceOwnership{ManagedByOperator: true, CRNamespace: "infra", CRName: "rabbit-main"}
		assert.NoError(t, setOwnership(ctx, baseDao, &model.RabbitInstance{}, "owned", ownership))

		_, err = rabbitDao.UpdateInstanceRegistration(ctx, &model.RabbitInstance{
			Id: "owned", ApiUrl: "http://rabbit:15672/api", AmqpUrl: "amqp://rabbit:5672", User: "u2", Password: "p2", Default: true,
		})
		assert.NoError(t, err)

		stored, err = rabbitDao.GetInstanceById(ctx, "owned")
		assert.NoError(t, err)
		assert.Equal(t, "u2", stored.User)
		assert.Equal(t, ownership, stored.InstanceOwnership)
	})
}

func setOwnership(ctx context.Context, baseDao *dao.BaseDaoImpl, table any, id string, ownership model.InstanceOwnership) error {
	return baseDao.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Model(table).Where("id=?", id).Updates(map[string]any{
			"managed_by_operator": ownership.ManagedByOperator,
			"cr_namespace":        ownership.CRNamespace,
			"cr_name":             ownership.CRName,
		}).Error
	})
}
