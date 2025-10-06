package monitoring

import (
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestEntityRequestStatDto_AsModel(t *testing.T) {
	dto := &EntityRequestStatDto{
		Namespace:     "test-namespace",
		Microservice:  "test-microservice",
		Name:          "test-topic",
		EntityType:    EntityTypeTopic,
		lastRequestTs: time.Now(),
		RequestsTotal: 42,
	}

	model := dto.AsModel()
	assert.Equal(t, dto.Namespace, model.Namespace)
	assert.Equal(t, dto.Microservice, model.Microservice)
	assert.Equal(t, dto.Name, model.Name)
	assert.Equal(t, dto.EntityType, model.EntityType)
	assert.Equal(t, dto.lastRequestTs, model.LastRequestTs)
	assert.Equal(t, dto.RequestsTotal, model.RequestsTotal)
}

func TestEntityRequestStatDto_AsDto(t *testing.T) {
	model := &EntityRequestStat{
		Namespace:     "test-namespace",
		Microservice:  "test-microservice",
		Name:          "test-topic",
		EntityType:    EntityTypeTopic,
		LastRequestTs: time.Now(),
		RequestsTotal: 42,
	}

	dto := model.AsDto()
	assert.Equal(t, model.Namespace, dto.Namespace)
	assert.Equal(t, model.Microservice, dto.Microservice)
	assert.Equal(t, model.Name, dto.Name)
	assert.Equal(t, model.EntityType, dto.EntityType)
	assert.Equal(t, model.LastRequestTs, dto.lastRequestTs)
	assert.Equal(t, model.RequestsTotal, dto.RequestsTotal)
}
