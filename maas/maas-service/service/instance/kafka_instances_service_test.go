package instance

import (
	"context"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	mock_instance "github.com/netcracker/qubership-maas/service/instance/mock"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	assertion *assert.Assertions
	mockCtrl  *gomock.Controller
	ctx       context.Context
	mockDao   *mock_instance.MockKafkaInstancesDao
)

func testInitializer(t *testing.T) {
	ctx = context.Background()
	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	mockDao = mock_instance.NewMockKafkaInstancesDao(mockCtrl)
}

func TestKafkaInstanceRegistration_Register(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	resultInstance := model.KafkaInstance{
		Id:           "test_kafka",
		MaasProtocol: model.Plaintext,
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Eq(&resultInstance)).
			Return(&resultInstance, nil).
			Times(1),
	)
	service := NewKafkaInstanceServiceWithHealthChecker(
		mockDao,
		nil,
		func(kafkaInstance *model.KafkaInstance) error {
			return nil
		},
	)
	_, err := service.Register(ctx, &instance)
	assertion.Nil(err)
}

func TestKafkaInstanceRegistration_Update(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092", "localhost:9093"}},
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			UpdateInstanceRegistration(eqCtx, gomock.Eq(&instance)).
			Return(&instance, nil).
			Times(1),
	)
	service := NewKafkaInstanceServiceWithHealthChecker(
		mockDao,
		nil,
		func(kafkaInstance *model.KafkaInstance) error {
			return nil
		},
	)
	_, err := service.Update(ctx, &instance)
	assertion.Nil(err)
}

func TestKafkaInstanceRegistration_UpdateForInvalidkafkaInstance(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.KafkaInstance{
		Id:        "test",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092", "localhost:9093"}},
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			UpdateInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, errors.New("some dao error")).
			Times(1))
	service := NewKafkaInstanceServiceWithHealthChecker(
		mockDao,
		nil,
		func(kafkaInstance *model.KafkaInstance) error {
			return nil
		},
	)
	_, err := service.Update(ctx, &instance)
	assertion.Error(err, "Dao:Kafka Instance is not Found")
}

func TestKafkaInstanceRegistration_UpdateForDefaultInstance(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	service := NewKafkaInstanceService(mockDao, nil)
	_, err := service.Update(ctx, &model.KafkaInstance{})
	assertion.Error(err, "Dao:Kafka Instance is not Found")
}

func TestKafkaInstanceRegistration_Unregister(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092", "localhost:9093"}},
	}

	mockDao := mock_instance.NewMockKafkaInstancesDao(mockCtrl)
	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			RemoveInstanceRegistration(eqCtx, gomock.Eq(instance.Id)).
			Return(instance, nil).
			Times(1),
	)
	service := NewKafkaInstanceService(mockDao, nil)
	removed, err := service.Unregister(ctx, instance.GetId())
	assertion.NoError(err, nil)
	assertion.Equal(instance, removed)
}

func TestKafkaInstanceServiceImpl_Register(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.KafkaInstance{
		Id:        "test_wrong_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"wrong:9092"}},
		// missed MAAS_PROTOCOL value, so health routine can't get broker addresses
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(0),
	)
	service := NewKafkaInstanceService(mockDao, nil)
	_, err := service.Register(ctx, &instance)
	assertion.Error(err)

	assertion.ErrorIs(err, msg.BadRequest)
}

func TestKafkaInstanceServiceImpl_Update(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.KafkaInstance{
		Id:        "test_wrong_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"wrong:9092"}},
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(0),
	)
	service := NewKafkaInstanceService(mockDao, nil)
	_, err := service.Update(ctx, &instance)
	assertion.Error(err)

	assertion.ErrorIs(err, msg.BadRequest)
}
