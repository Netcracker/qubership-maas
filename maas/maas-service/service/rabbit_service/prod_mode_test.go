package rabbit_service

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/msg"
	mock_rabbit_service "github.com/netcracker/qubership-maas/service/rabbit_service/mock"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProdMode_RemoveVHosts(t *testing.T) {
	pm := NewProdMode(nil, true)
	assert.ErrorIs(t, pm.RemoveVHosts(nil, nil, ""), msg.BadRequest)
}

func TestProdMode_RemoveVHost_Allow(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	service := mock_rabbit_service.NewMockRabbitService(mockCtrl)

	service.EXPECT().RemoveVHosts(gomock.Eq(ctx), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	pm := NewProdMode(service, false)
	assert.NoError(t, pm.RemoveVHosts(ctx, nil, ""))
}
