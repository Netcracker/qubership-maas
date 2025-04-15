package kafka

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestProdMode_DeleteTopic(t *testing.T) {
	pm := NewProdMode(nil, true)
	assert.ErrorIs(t, pm.DeleteTopic(nil, nil, false), msg.BadRequest)
}

func TestProdMode_DeleteTopic_Allow(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	service := NewMockKafkaService(mockCtrl)

	service.EXPECT().DeleteTopic(gomock.Eq(ctx), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	pm := NewProdMode(service, false)
	assert.NoError(t, pm.DeleteTopic(ctx, nil, false))
}

func TestProdMode_DeleteTopics(t *testing.T) {
	pm := NewProdMode(nil, true)
	_, err := pm.DeleteTopics(nil, nil)
	assert.ErrorIs(t, err, msg.BadRequest)
}

func TestProdMode_DeleteTopics_Allow(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	service := NewMockKafkaService(mockCtrl)

	service.EXPECT().DeleteTopics(gomock.Eq(ctx), gomock.Any()).Return(nil, nil).Times(1)

	pm := NewProdMode(service, false)
	_, err := pm.DeleteTopics(ctx, nil)
	assert.NoError(t, err)
}

func TestProdMode_DeleteTopicDefinition(t *testing.T) {
	pm := NewProdMode(nil, true)
	_, err := pm.DeleteTopicDefinition(nil, nil)
	assert.ErrorIs(t, err, msg.BadRequest)
}

func TestProdMode_DeleteTopicDefinition_Allow(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	service := NewMockKafkaService(mockCtrl)

	service.EXPECT().DeleteTopicDefinition(gomock.Eq(ctx), gomock.Any()).Return(nil, nil).Times(1)

	pm := NewProdMode(service, false)
	_, err := pm.DeleteTopicDefinition(ctx, nil)
	assert.NoError(t, err)
}

func TestProdMode_DeleteTopicTemplate(t *testing.T) {
	pm := NewProdMode(nil, true)
	_, err := pm.DeleteTopicTemplate(nil, "", "")
	assert.ErrorIs(t, err, msg.BadRequest)
}

func TestProdMode_DeleteTopicTemplate_Allow(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	service := NewMockKafkaService(mockCtrl)

	service.EXPECT().DeleteTopicTemplate(gomock.Eq(ctx), gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)

	pm := NewProdMode(service, false)
	_, err := pm.DeleteTopicTemplate(ctx, "", "")
	assert.NoError(t, err)
}
