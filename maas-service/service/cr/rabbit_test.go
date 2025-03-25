package cr

import (
	"github.com/stretchr/testify/assert"
	"maas/maas-service/model"
	"testing"
)

func Test_adaptVhostConfig(t *testing.T) {
	entities := model.RabbitEntities{
		Exchanges: []interface{}{CustomResourceSpecRequest{"name": "test-exchange", "type": "direct", "durable": "true"}},
		Queues:    []interface{}{CustomResourceSpecRequest{"name": "test-queue", "durable": "true"}},
		Bindings:  []interface{}{CustomResourceSpecRequest{"source": "test-exchange", "destination": "test-queue", "routing_key": "key"}},
	}

	config, err := adaptVhostConfig(&RabbitVhostConfigSpec{
		Classifier: &RabbitVhostConfigSpecClassifier{Name: "name-from-spec"},
		Entities:   &entities,
	}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "name-from-spec", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)

	config, err = adaptVhostConfig(&RabbitVhostConfigSpec{
		Entities: &entities,
	}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "name-from-metadata", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)
}
