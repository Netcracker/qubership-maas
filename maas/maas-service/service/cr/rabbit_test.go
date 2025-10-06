package cr

import (
	"testing"

	"github.com/netcracker/qubership-maas/model"
	"github.com/stretchr/testify/assert"
)

func Test_adaptVhostConfig(t *testing.T) {
	entities := model.RabbitEntities{
		Exchanges: []interface{}{CustomResourceSpecRequest{"name": "test-exchange", "type": "direct", "durable": "true"}},
		Queues:    []interface{}{CustomResourceSpecRequest{"name": "test-queue", "durable": "true"}},
		Bindings:  []interface{}{CustomResourceSpecRequest{"source": "test-exchange", "destination": "test-queue", "routing_key": "key"}},
	}

	// Test case 1: Configuration with name from spec
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
	assert.Nil(t, config.Spec.RabbitDeletions)

	// Test case 2: Configuration with name from metadata
	config, err = adaptVhostConfig(&RabbitVhostConfigSpec{
		Entities: &entities,
	}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "name-from-metadata", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)
	assert.Nil(t, config.Spec.RabbitDeletions)

	// Test case 3: Configuration with deletions containing both entities and policies
	policies := []interface{}{
		CustomResourceSpecRequest{"name": "create-policy", "pattern": "test.*", "definition": map[string]interface{}{"max-length": 500}},
	}

	deletionPolicies := []interface{}{
		CustomResourceSpecRequest{"name": "delete-policy", "pattern": ".*", "definition": map[string]interface{}{"max-length": 1000}},
	}

	deletions := model.RabbitDeletions{
		RabbitEntities: model.RabbitEntities{
			Exchanges: []interface{}{CustomResourceSpecRequest{"name": "delete-exchange", "type": "direct"}},
			Queues:    []interface{}{CustomResourceSpecRequest{"name": "delete-queue"}},
			Bindings:  []interface{}{CustomResourceSpecRequest{"source": "delete-exchange", "destination": "delete-queue", "routing_key": "delete-key"}},
		},
		RabbitPolicies: model.RabbitPolicies{
			Policies: deletionPolicies,
		},
	}

	config, err = adaptVhostConfig(&RabbitVhostConfigSpec{
		Classifier: &RabbitVhostConfigSpecClassifier{Name: "test-with-deletions"},
		Entities:   &entities,
		Deletions:  &deletions,
		Policies:   policies,
	}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "test-with-deletions", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)
	assert.NotNil(t, config.Spec.RabbitDeletions)

	assert.Equal(t, 1, len(config.Spec.RabbitDeletions.Exchanges))
	assert.Equal(t, 1, len(config.Spec.RabbitDeletions.Queues))
	assert.Equal(t, 1, len(config.Spec.RabbitDeletions.Bindings))

	deletionExchange := config.Spec.RabbitDeletions.Exchanges[0].(map[string]interface{})
	assert.Equal(t, "delete-exchange", deletionExchange["name"])
	assert.Equal(t, "direct", deletionExchange["type"])

	deletionQueue := config.Spec.RabbitDeletions.Queues[0].(map[string]interface{})
	assert.Equal(t, "delete-queue", deletionQueue["name"])

	deletionBinding := config.Spec.RabbitDeletions.Bindings[0].(map[string]interface{})
	assert.Equal(t, "delete-exchange", deletionBinding["source"])
	assert.Equal(t, "delete-queue", deletionBinding["destination"])
	assert.Equal(t, "delete-key", deletionBinding["routing_key"])

	assert.Equal(t, 1, len(config.Spec.RabbitPolicies))
	policy := config.Spec.RabbitPolicies[0].(CustomResourceSpecRequest)
	assert.Equal(t, "create-policy", policy["name"])
	assert.Equal(t, "test.*", policy["pattern"])

	assert.Equal(t, 1, len(config.Spec.RabbitDeletions.Policies))
	deletionPolicy := config.Spec.RabbitDeletions.Policies[0].(map[string]interface{})
	assert.Equal(t, "delete-policy", deletionPolicy["name"])
	assert.Equal(t, ".*", deletionPolicy["pattern"])

}
