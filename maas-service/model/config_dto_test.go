package model

import (
	"encoding/json"
	"github.com/ghodss/yaml"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestPragmaUnmarshalDefault(t *testing.T) {
	var e ConfigPragmaDto
	assert.NoError(t, json.Unmarshal([]byte(`{}`), &e))
	assert.Equal(t, ConfigPragmaDto{OnEntityExists: Fail}, e)
}

func TestTopicRegistrationConfigReqDto_Correct(t *testing.T) {
	sample := `
apiVersion: nc.maas.kafka/v1
kind: topic
pragma:
  kube-secret: secret-name
  on-entity-exists: merge 
spec:
  classifier:
    name: "abc" 
`
	expected := TopicRegistrationConfigReqDto{
		Kind: Kind{
			ApiVersion: "nc.maas.kafka/v1",
			Kind:       "topic",
		},
		Pragma: &ConfigPragmaDto{
			KubeSecret:     "secret-name",
			OnEntityExists: Merge,
		},
		Spec: TopicRegistrationReqDto{
			Classifier: Classifier{
				Name: "abc",
			},
		},
	}

	var actual TopicRegistrationConfigReqDto
	assert.NoError(t, yaml.Unmarshal([]byte(sample), &actual))
	assert.Equal(t, expected, actual)
}

func TestTopicRegistrationConfigReqDto_EmptyPragma(t *testing.T) {
	obj := TopicRegistrationConfigReqDto{
		Kind: Kind{
			ApiVersion: "nc.maas.kafka/v1",
			Kind:       "topic",
		},
		Spec: TopicRegistrationReqDto{
			Classifier: Classifier{Name: "abc:"},
		},
	}

	actual, err := json.Marshal(obj)
	assert.NoError(t, err)
	assert.Equal(t, string(actual), `{"apiVersion":"nc.maas.kafka/v1","kind":"topic","spec":{"classifier":{"name":"abc:"}}}`)
}

func TestTopicTemplate_JsonMarshal_DomainNamespacesAbsentsInResult(t *testing.T) {
	customFlushMs := "100"
	var numPartitions int32 = 3
	var replicationFactor int16 = 3
	settings := TopicSettings{
		NumPartitions:     &numPartitions,
		ReplicationFactor: &replicationFactor,
		ReplicaAssignment: map[int32][]int32{0: {0, 1, 2}, 1: {2, 0, 1}, 2: {0, 1, 2}},
		Configs:           map[string]*string{"flush.ms": &customFlushMs},
	}

	template := &TopicTemplate{
		Id:               123,
		Name:             "topic-name",
		Namespace:        "topic-name-space",
		TopicSettings:    settings,
		DomainNamespaces: []string{"topic-domain-name-space"},
	}

	actual, err := json.Marshal(template)
	assert.NoError(t, err)
	assert.NotContains(t, string(actual), "topic-domain-name-space")
}
