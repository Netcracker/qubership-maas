package monitoring

import (
	"context"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/watchdog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func Test_ActiveAuditor_AddEntityRequestStat(t *testing.T) {
	assertions := require.New(t)
	requestContext := &model.RequestContext{
		Namespace:    "test-namespace",
		Microservice: "test-microservice",
	}
	ctx := model.WithRequestContext(context.Background(), requestContext)
	req := EntityRequestStatDto{
		Namespace:    "test-namespace",
		Microservice: "test-microservice",
		Name:         "test-topic",
		EntityType:   EntityTypeTopic,
		Instance:     "core-dev",
	}
	mockCtrl := gomock.NewController(t)

	wg := &utils.WaitGroupWithTimeout{}
	wg.Add(1)

	auditDao := NewMockDao(mockCtrl)
	auditDao.EXPECT().
		AddEntityRequestStat(gomock.Any(), req.AsModel()).
		DoAndReturn(func(ctx, f any) error {
			wg.Done()
			return nil
		}).
		Times(1)

	auditor := NewAuditor(auditDao, dr.Active, nil)
	auditor.AddEntityRequestStat(ctx, EntityTypeTopic, req.Name, "core-dev")

	assertions.True(wg.Wait(5*time.Second), "timed out waiting for async AddEntityRequestStat call")
}

func Test_DisabledAuditor_AddEntityRequestStat(t *testing.T) {
	requestContext := &model.RequestContext{
		Namespace:    "test-namespace",
		Microservice: "test-microservice",
	}
	ctx := model.WithRequestContext(context.Background(), requestContext)
	req := EntityRequestStatDto{
		Namespace:    "test-namespace",
		Microservice: "test-microservice",
		Name:         "test-topic",
		EntityType:   EntityTypeTopic,
		Instance:     "core-dev",
	}
	mockCtrl := gomock.NewController(t)

	auditDao := NewMockDao(mockCtrl)

	auditor := NewAuditor(auditDao, dr.Disabled, nil)
	auditor.AddEntityRequestStat(ctx, EntityTypeTopic, req.Name, "core-dev")
}

func TestDefaultAuditor_GetAllEntityRequestsStat(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	auditDao := NewMockDao(mockCtrl)
	sqlError := errors.New("oops")
	auditDao.EXPECT().
		GetAllEntityRequestsStat(gomock.Any()).
		Return(nil, sqlError).
		Times(1)
	auditDao.EXPECT().
		GetAllEntityRequestsStat(gomock.Any()).
		Return(&[]EntityRequestStat{
			{
				Namespace:     "core-dev",
				Microservice:  "order-proc",
				EntityType:    EntityTypeTopic,
				Name:          "core-dev-orders",
				Instance:      "kafka-default",
				LastRequestTs: time.Now(),
				RequestsTotal: 10,
			},
		}, nil).
		Times(1)

	auditor := NewAuditor(auditDao, dr.Active, nil)
	res, err := auditor.GetAllEntityRequestsStat(ctx)
	assert.ErrorIs(t, err, sqlError)

	res, err = auditor.GetAllEntityRequestsStat(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(*res))
}

func TestDefaultAuditor_GetKafkaMonitoringEntities(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	auditDao := NewMockDao(mockCtrl)
	sqlError := errors.New("oops")
	auditDao.EXPECT().
		GetMonitoringTopics(gomock.Any()).
		Return(nil, sqlError).
		Times(1)
	auditDao.EXPECT().
		GetMonitoringTopics(gomock.Any()).
		Return(&[]MonitoringTopic{
			{
				Namespace:  "core-dev",
				BrokerHost: "kafka-server",
				Topic:      "core-dev-orders",
				InstanceID: "default",
			},
		}, nil).
		Times(1)

	auditor := NewAuditor(auditDao, dr.Active, func(string) watchdog.Status { return watchdog.UP })
	res, err := auditor.GetKafkaMonitoringEntities(ctx)
	assert.ErrorIs(t, err, sqlError)

	res, err = auditor.GetKafkaMonitoringEntities(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(*res))
}

func TestDefaultAuditor_GetRabbitMonitoringEntities(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	auditDao := NewMockDao(mockCtrl)
	sqlError := errors.New("oops")
	auditDao.EXPECT().
		GetMonitoringVhosts(gomock.Any()).
		Return(nil, sqlError).
		Times(1)
	auditDao.EXPECT().
		GetMonitoringVhosts(gomock.Any()).
		Return(&[]MonitoringVhost{
			{
				Namespace:    "core-dev",
				Microservice: "bill-processor",
				BrokerHost:   "kafka-server",
				Vhost:        "core-dev_public",
				InstanceID:   "default",
			},
		}, nil).
		Times(1)

	auditor := NewAuditor(auditDao, dr.Active, func(string) watchdog.Status { return watchdog.UP })
	res, err := auditor.GetRabbitMonitoringEntities(ctx)
	assert.ErrorIs(t, err, sqlError)

	res, err = auditor.GetRabbitMonitoringEntities(ctx)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(*res))
}

func TestDefaultAuditor_CleanupData(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	auditDao := NewMockDao(mockCtrl)
	auditDao.EXPECT().
		DeleteEntityRequestsStatByNamespace(gomock.Any(), gomock.Eq("core-dev")).
		Return(nil).
		Times(1)

	auditor := NewAuditor(auditDao, dr.Active, nil)
	err := auditor.CleanupData(ctx, "core-dev")
	assert.NoError(t, err)
}
