package watchdog

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	mock_watchdog "github.com/netcracker/qubership-maas/watchdog/mock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestUtils_1(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	mockCtrl := gomock.NewController(t)
	ik := mock_watchdog.NewMockKafkaInstanceProvider(mockCtrl)
	ir := mock_watchdog.NewMockRabbitInstanceProvider(mockCtrl)

	ik.EXPECT().AddInstanceUpdateCallback(gomock.Any()).Times(1)
	ir.EXPECT().AddInstanceUpdateCallback(gomock.Any()).Times(1)
	ik.EXPECT().GetKafkaInstances(gomock.Any()).Return(&[]model.KafkaInstance{{Id: "kf"}}, nil).AnyTimes()
	ir.EXPECT().GetRabbitInstances(gomock.Any()).Return(&[]model.RabbitInstance{{Id: "rb"}}, nil).AnyTimes()
	ik.EXPECT().CheckHealth(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	ir.EXPECT().CheckHealth(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	m := NewBrokerInstancesMonitor(ik, ir, 100*time.Millisecond)
	assert.Equal(t, UNKNOWN, m.GetStatus("kf"))
	m.Start(ctx)
	// emulate instance change
	m.enqueueKafkaInstanceListUpdate(ctx, nil)
	m.enqueueRabbitInstanceListUpdate(ctx, nil)
	utils.CancelableSleep(ctx, 200*time.Millisecond)

	assert.ElementsMatch(t, []BrokerStatus{
		{BrokerType: "Kafka", Status: StatusDetailed{Status: UP}},
		{BrokerType: "RabbitMQ", Status: StatusDetailed{Status: UP}},
	}, m.All())
	assert.Equal(t, UNKNOWN, m.GetStatus("none"))
	assert.Equal(t, UP, m.GetStatus("kf"))

}
