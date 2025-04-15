package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-maas/msg"
)

type OnEntityExistsEnum int

const (
	Fail OnEntityExistsEnum = iota
	Merge
)

const (
	STATUS_ERROR    = "error"
	STATUS_OK       = "ok"
	INITIAL_VERSION = "v1"
)

func (e *OnEntityExistsEnum) UnmarshalJSON(b []byte) error {
	var value string
	if err := json.Unmarshal(b, &value); err != nil {
		return errors.New(fmt.Sprintf("Error unmarshal value for `on-entity-exists` field: `%v'", err))
	}
	switch value {
	case "merge":
		*e = Merge
	case "fail", "":
		*e = Fail
	default:
		return errors.New(fmt.Sprintf("Invalid value for `on-entity-exists' field: `%v'. Available values: `merge', `fail'", string(b)))
	}
	return nil
}

type ConfigPragmaDto struct {
	KubeSecret     string             `json:"kube-secret,omitempty"`
	OnEntityExists OnEntityExistsEnum `json:"on-entity-exists,omitempty"`
}

type Kind struct {
	ApiVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
}

type ConfigReqDto struct {
	Kind
	Pragma *ConfigPragmaDto `json:"pragma,omitempty"`
	Spec   ConfigReqDtoSpec `json:"spec"`
}

type ConfigReqDtoSpec struct {
	Version        string          `json:"version"`
	Namespace      string          `json:"namespace"`
	BaseNamespace  string          `json:"base-namespace"`
	SharedConfigs  interface{}     `json:"shared"`
	ServiceConfigs []ServiceConfig `json:"services"`
}

type ServiceConfigs struct {
	Version        string
	Namespace      string
	ServiceConfigs []ServiceConfig
}

type ServiceConfig struct {
	ServiceName string      `json:"serviceName"`
	Config      interface{} `json:"config"`
}

type ConfigApplicationResponse struct {
	Status      string             `json:"status"`
	Error       string             `json:"error,omitempty"`
	MsResponses []ConfigMsResponse `json:"msResponses"`
}

type ConfigMsResponse struct {
	Request interface{}    `json:"request"`
	Result  ConfigMsResult `json:"result"`
}

type ConfigMsResult struct {
	Status string      `json:"status"`
	Error  string      `json:"error,omitempty"`
	Data   interface{} `json:"data,omitempty"`
}

type InstanceDesignatorKafkaReq struct {
	Kind
	Spec InstanceDesignatorKafka `json:"spec"`
}

type InstanceDesignatorRabbitReq struct {
	Kind
	Spec InstanceDesignatorRabbit `json:"spec"`
}

type RabbitConfigReqDto struct {
	Kind
	Pragma *ConfigPragmaDto        `json:"pragma,omitempty"`
	Spec   ApplyRabbitConfigReqDto `json:"spec"`
}

type ApplyRabbitConfigReqDto struct {
	Classifier        Classifier       `json:"classifier" validate:"required"`
	InstanceId        string           `json:"instanceId,omitempty"`
	VersionedEntities *RabbitEntities  `json:"versionedEntities,omitempty"`
	Entities          *RabbitEntities  `json:"entities,omitempty"`
	RabbitPolicies    []interface{}    `json:"policies,omitempty"`
	RabbitDeletions   *RabbitDeletions `json:"deletions,omitempty"`
}

type TopicRegistrationConfigReqDto struct {
	Kind
	Pragma *ConfigPragmaDto        `json:"pragma,omitempty"`
	Spec   TopicRegistrationReqDto `json:"spec"`
}

type TopicTemplateDeleteConfig struct {
	Kind
	Spec *TopicTemplateDeleteCriteria `json:"spec"`
}

type TopicDeleteConfig struct {
	Kind
	Spec *TopicDeleteCriteria `json:"spec"`
}

type TopicDeleteCriteria struct {
	Classifier           Classifier `json:"classifier"`
	LeaveRealTopicIntact bool       `json:"leaveRealTopicIntact"`
}

type TopicTemplateDeleteCriteria struct {
	Name string `json:"name"`
}

type TopicTemplateConfigReqDto struct {
	Kind
	Pragma *ConfigPragmaDto    `json:"pragma,omitempty"`
	Spec   TopicTemplateReqDto `json:"spec"`
}

type TopicTemplateReqDto struct {
	Name          string `json:"name"`
	TopicSettings `mapstructure:",squash"`
}

func (topicTemplateReqDto TopicTemplateReqDto) BuildTopicTemplateFromReq(namespace string) (*TopicTemplate, error) {
	// TODO use validator for all these checks
	if topicTemplateReqDto.Name == "" {
		return nil, fmt.Errorf("topic template name shouldn't be empty: %w", msg.BadRequest)
	}
	if topicTemplateReqDto.NumPartitions != nil && topicTemplateReqDto.MinNumPartitions != nil {
		return nil, fmt.Errorf("`numPartitions' property is mutual exclusive with `minNumPartitions': %w", msg.BadRequest)
	}
	numPartitions := topicTemplateReqDto.NumPartitions
	replicationFactor := topicTemplateReqDto.ReplicationFactor
	minNumPartitions := topicTemplateReqDto.MinNumPartitions

	if numPartitions == nil && minNumPartitions == nil {
		numPartitions = new(int32)
		*numPartitions = 1
	}
	if replicationFactor == nil {
		replicationFactor = new(int16)
		*replicationFactor = 1
	}

	return &TopicTemplate{
		Name:      topicTemplateReqDto.Name,
		Namespace: namespace,
		TopicSettings: TopicSettings{
			NumPartitions:     numPartitions,
			MinNumPartitions:  minNumPartitions,
			ReplicationFactor: replicationFactor,
			ReplicaAssignment: topicTemplateReqDto.ReplicaAssignment,
			Configs:           topicTemplateReqDto.Configs,
		},
	}, nil
}

type TopicTemplateRespDto struct {
	Name             string                     `json:"name"`
	Namespace        string                     `json:"namespace"`
	CurrentSettings  *TopicSettings             `json:"currentSettings"`
	PreviousSettings *TopicSettings             `json:"previousSettings,omitempty"`
	UpdatedTopics    []TopicRegistrationRespDto `json:"updatedTopics,omitempty"`
}
