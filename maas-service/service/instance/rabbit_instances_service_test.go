package instance

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	mock_instance "maas/maas-service/service/instance/mock"
	"testing"
)

var mockRabbitDao *mock_instance.MockRabbitInstancesDao

func rabbitTestInitializer(t *testing.T) {
	ctx = context.Background()
	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	mockRabbitDao = mock_instance.NewMockRabbitInstancesDao(mockCtrl)
}

func TestRabbitInstanceOperations(t *testing.T) {
	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		ctx := context.Background()

		dao := NewRabbitInstancesDao(base)
		defer base.Close()

		main := &model.RabbitInstance{
			Id:       "main",
			ApiUrl:   "http://localhost:15432/api",
			AmqpUrl:  "amqp://localhost:5432",
			User:     "admin",
			Password: "admin",
			Default:  false,
		}
		manager := NewRabbitInstanceServiceWithHealthChecker(
			dao,
			func(rabbitInstance *model.RabbitInstance) error {
				return nil
			},
		)
		inserted, err := manager.Register(ctx, main)
		assert.NoError(t, err)
		assert.True(t, inserted.Default)

		{ // test added instance
			inst, err := manager.GetById(ctx, "main")
			assert.NoError(t, err)
			assert.Equal(t, "main", inst.GetId())

			// this is only one registered instance, it should be set default ignoring request definition
			assert.True(t, inst.IsDefault())
		}

		// add another registration marked as new default
		backupInstance := &model.RabbitInstance{
			Id:       "backup",
			ApiUrl:   "http://other:15432/api",
			AmqpUrl:  "amqp://olther:5432",
			User:     "admin",
			Password: "admin",
			Default:  true,
		}
		backup, err := manager.Register(ctx, backupInstance)
		assert.NoError(t, err)
		assert.True(t, backup.Default)

		{
			inst, err := manager.GetDefault(ctx)
			assert.NoError(t, err)
			assert.Equal(t, backup, inst)
		}

		{
			inst, err := manager.GetById(ctx, "main")
			assert.NoError(t, err)
			assert.Equal(t, main.Id, inst.Id)
			// first main server should lose it default status
			assert.False(t, inst.IsDefault())
		}

		{
			main.AmqpUrl = "amqp://0.0.0.0:7484"
			main.Default = false
			result, err := manager.Update(ctx, main)
			assert.NoError(t, err)
			assert.Equal(t, main, result)
		}

		// test update
		// check error delete default instance
		_, err = manager.Unregister(ctx, "backup")
		assert.Error(t, err)

		// delete non default instance
		inst, err := manager.Unregister(ctx, "main")
		assert.NoError(t, err)
		assert.Equal(t, inst.Id, main.Id)

		// because this is last instance registration, its removal should be fine
		_, err = manager.Unregister(ctx, "backup")
		assert.NoError(t, err)
	})
}

func TestRabbitInstanceServiceImpl_Register(t *testing.T) {
	rabbitTestInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:      "test_wrong_rabbit",
		ApiUrl:  "https://wrong:15671/api",
		AmqpUrl: "amqps://wrong:5671",
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockRabbitDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(0),
	)
	service := NewRabbitInstanceService(mockRabbitDao)
	_, err := service.Register(ctx, &instance)
	assertion.Error(err)

	assertion.ErrorIs(err, msg.BadRequest)
}

func TestRabbitInstanceServiceImpl_Update(t *testing.T) {
	rabbitTestInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:      "test_wrong_rabbit",
		ApiUrl:  "https://wrong:15671/api",
		AmqpUrl: "amqps://wrong:5671",
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockRabbitDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(0),
	)
	service := NewRabbitInstanceService(mockRabbitDao)
	_, err := service.Update(ctx, &instance)
	assertion.Error(err)

	assertion.ErrorIs(err, msg.BadRequest)
}
