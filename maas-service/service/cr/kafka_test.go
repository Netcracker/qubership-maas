package cr

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_adaptTopicConfig_override_name(t *testing.T) {
	config, err := adaptTopicConfig(&KafkaTopicConfig{
		Classifier: &KafkaTopicConfigClassifier{Name: "name-from-spec"},
	}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "name-from-spec", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)

	config, err = adaptTopicConfig(&KafkaTopicConfig{}, &CustomResourceMetadataRequest{
		Name:      "name-from-metadata",
		Namespace: "test-namespace",
	})
	assert.NoError(t, err)
	assert.Equal(t, "name-from-metadata", config.Spec.Classifier.Name)
	assert.Equal(t, "test-namespace", config.Spec.Classifier.Namespace)
}
