package model

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"maas/maas-service/msg"
	"maas/maas-service/utils"
	"reflect"
	"strconv"

	"github.com/pkg/errors"
)

type ClientAccountDto struct {
	Username  string             `json:"username" example:"client"`
	Password  utils.SecretString `json:"password" example:"client"`
	Namespace string             `json:"namespace" example:"namespace"`
	Roles     []RoleName         `json:"roles" example:"agent"`
}

type ManagerAccountDto struct {
	Username string             `json:"username" example:"manager"`
	Password utils.SecretString `json:"password" example:"24002eadbc"`
}

type VHostRegistrationResponse struct {
	Cnn      string `json:"cnn" example:"amqp://127.0.0.1:5672/namespace.test12"`
	Username string `json:"username" example:"9757343a78a04057a07ee4215f1b1355"`
	Password string `json:"password" example:"plain:504a449065924332b062c4a84e830cbe"`
	ApiUrl   string `json:"apiUrl,omitempty" example:"http://127.0.0.1:15672/api"`
}

type VHostRegistrationReqDto struct {
	Classifier Classifier `json:"classifier" validate:"required"`
	Instance   string     `json:"instance" example:"<[optional] rabbitmq instance for virtual host. if not specified default instance will be used>"`
}

type TopicRegistrationReqDto struct {
	Name              string             `json:"name,omitempty" validate:"required_with=ExternallyManaged" mapstructure:"topicNameTemplate"`
	Classifier        Classifier         `json:"classifier" validate:"required"`
	ExternallyManaged bool               `json:"externallyManaged,omitempty"`
	Instance          string             `json:"instance,omitempty"`
	NumPartitions     interface{}        `json:"numPartitions,omitempty" validate:"omitempty,excluded_with=MinNumPartitions"`
	MinNumPartitions  interface{}        `json:"minNumPartitions,omitempty" validate:"omitempty,excluded_with=NumPartitions"`
	ReplicationFactor interface{}        `json:"replicationFactor,omitempty"`
	ReplicaAssignment map[int32][]int32  `json:"replicaAssignment,omitempty"`
	Configs           map[string]*string `json:"configs,omitempty"`
	Template          string             `json:"template,omitempty"`
	Versioned         bool               `json:"versioned,omitempty"`
}

func (topicRegistrationReqDto *TopicRegistrationReqDto) BuildTopicRegFromReq(topicTemplate *TopicTemplate) (*TopicRegistration, error) {
	topicSettings := TopicSettings{}

	var templateId int
	if topicRegistrationReqDto.Template != "" {
		if topicTemplate == nil {
			return nil, fmt.Errorf("no topic template was found by name: %v and namespace: %v: error: %w", topicRegistrationReqDto.Template, topicRegistrationReqDto.Classifier.Namespace, msg.BadRequest)
		} else {
			topicSettings = *topicTemplate.GetSettings()
			templateId = topicTemplate.Id
		}
	} else {
		var err error
		topicSettings.NumPartitions, err = getNumPartitionsFromRequest(topicRegistrationReqDto.NumPartitions)
		if err != nil {
			return nil, err
		}
		topicSettings.MinNumPartitions, err = getMinNumPartitionsFromRequest(topicRegistrationReqDto.MinNumPartitions)
		if err != nil {
			return nil, err
		}
		topicSettings.ReplicationFactor, err = getReplicationFactorFromRequest(topicRegistrationReqDto.ReplicationFactor)
		if err != nil {
			return nil, err
		}
		topicSettings.ReplicaAssignment = topicRegistrationReqDto.ReplicaAssignment
		topicSettings.Configs = topicRegistrationReqDto.Configs
		topicSettings.Versioned = topicRegistrationReqDto.Versioned
	}

	var templateNullable sql.NullInt64
	if templateId == 0 {
		templateNullable = sql.NullInt64{Int64: int64(templateId), Valid: false}
	} else {
		templateNullable = sql.NullInt64{Int64: int64(templateId), Valid: true}
	}

	regReq, err := json.Marshal(topicRegistrationReqDto)
	if err != nil {
		return nil, err
	}

	return &TopicRegistration{
		Classifier:        &topicRegistrationReqDto.Classifier,
		Topic:             topicRegistrationReqDto.Name,
		Instance:          topicRegistrationReqDto.Instance,
		Namespace:         topicRegistrationReqDto.Classifier.Namespace, //checked during name resolving
		ExternallyManaged: topicRegistrationReqDto.ExternallyManaged,
		TopicSettings:     topicSettings,
		Template:          templateNullable,
		CreateReq:         string(regReq),
	}, nil
}

func (topicRegistrationReqDto *TopicRegistrationReqDto) ToTopicDefinition(namespace, kind string) (*TopicDefinition, error) {
	var numPartitionInt int
	// TODO numPartitions and replicationFactor conversions should be in one place
	switch value := topicRegistrationReqDto.NumPartitions.(type) {
	case string, json.Number:
		var err error
		numPartitionInt, err = strconv.Atoi(fmt.Sprint(value))
		if err != nil {
			return nil, fmt.Errorf("cannot parse NumPartitions '%v': %w", value, err)
		}
	case int:
		numPartitionInt = value
	case float64:
		numPartitionInt = int(value)
	}

	var replFactorInt int
	switch value := topicRegistrationReqDto.ReplicationFactor.(type) {
	case string, json.Number:
		if value == "inherit" {
			replFactorInt = -1
		} else {
			var err error
			replFactorInt, err = strconv.Atoi(fmt.Sprint(value))
			if err != nil {
				return nil, fmt.Errorf("cannot parse ReplicationFactor '%v': %w", value, err)
			}
		}
	case int:
		replFactorInt = value
	case float64:
		replFactorInt = int(value)
	}

	var minNnumPartitionInt int
	switch value := topicRegistrationReqDto.MinNumPartitions.(type) {
	case string, json.Number:
		var err error
		minNnumPartitionInt, err = strconv.Atoi(fmt.Sprint(value))
		if err != nil {
			return nil, fmt.Errorf("cannot parse MinNumPartitions '%v': %w", value, err)
		}
	case int:
		minNnumPartitionInt = value
	case float64:
		minNnumPartitionInt = int(value)
	}

	return &TopicDefinition{
		Classifier:        &topicRegistrationReqDto.Classifier,
		Namespace:         namespace,
		Name:              topicRegistrationReqDto.Name,
		Instance:          topicRegistrationReqDto.Instance,
		NumPartitions:     numPartitionInt,
		MinNumPartitions:  minNnumPartitionInt,
		ReplicationFactor: replFactorInt,
		ReplicaAssignment: topicRegistrationReqDto.ReplicaAssignment,
		Configs:           topicRegistrationReqDto.Configs,
		Template:          topicRegistrationReqDto.Template,
		Kind:              kind,
		Versioned:         topicRegistrationReqDto.Versioned,
	}, nil
}

func getNumPartitionsFromRequest(value interface{}) (*int32, error) {
	numPartitionsStr := ""
	if value != nil {
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Int {
			numPartitionsStr = strconv.FormatInt(val.Elem().Int(), 10)
		} else {
			numPartitionsStr = fmt.Sprintf("%v", value)
		}
	}
	var numPartitions *int32 = nil
	// todo: think how to do it generic. Now "" is after parsing json, "0" after it is taken from db from TopicDefinition after migration to gorm
	if numPartitionsStr == "" || numPartitionsStr == "0" {
		numPartitions = new(int32)
		*numPartitions = int32(0)
	} else {
		// number value
		if n, err := strconv.Atoi(numPartitionsStr); err == nil {
			numPartitions = new(int32)
			*numPartitions = int32(n)
		} else {
			return nil, errors.Errorf("Can't parse `numPartitions' value: `%v'. Value can be only positive integer", numPartitionsStr)
		}
	}
	return numPartitions, nil
}

func getMinNumPartitionsFromRequest(value interface{}) (*int32, error) {
	numPartitionsStr := ""
	if value != nil {
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr && val.Elem().Kind() == reflect.Int {
			numPartitionsStr = strconv.FormatInt(val.Elem().Int(), 10)
		} else {
			numPartitionsStr = fmt.Sprintf("%v", value)
		}
	}
	var numPartitions *int32 = nil
	// todo: think how to do it generic. Now "" is after parsing json, "0" after it is taken from db from TopicDefinition after migration to gorm
	if numPartitionsStr == "" || numPartitionsStr == "0" {
		numPartitions = new(int32)
		*numPartitions = int32(0)
	} else {
		// number value
		if n, err := strconv.Atoi(numPartitionsStr); err == nil {
			numPartitions = new(int32)
			*numPartitions = int32(n)
		} else {
			return nil, errors.Errorf("Can't parse `numPartitions' value: `%v'. Value can be only positive integer", numPartitionsStr)
		}
	}
	return numPartitions, nil
}

func getReplicationFactorFromRequest(value interface{}) (*int16, error) {
	replicationFactorStr := ""
	if value != nil {
		val := reflect.ValueOf(value)
		if val.Kind() == reflect.Ptr {
			if val.Elem().Kind() == reflect.Int {
				replicationFactorStr = strconv.FormatInt(val.Elem().Int(), 10)
			} else {
				replicationFactorStr = fmt.Sprintf("%v", val.Elem())
			}
		} else {
			replicationFactorStr = fmt.Sprintf("%v", value)
		}
	}

	var replicationFactor *int16 = nil
	// todo: think how to do it generic. Now "" is after parsing json, "0" after it is taken from db from TopicDefinition after migration to gorm
	if replicationFactorStr == "" || replicationFactorStr == "0" {
		replicationFactor = new(int16)
		*replicationFactor = int16(0)
	} else if replicationFactorStr == "inherit" {
		replicationFactor = nil
	} else {
		// number value
		if n, err := strconv.Atoi(replicationFactorStr); err == nil {
			replicationFactor = new(int16)
			*replicationFactor = int16(n)
		} else {
			return nil, errors.Errorf("Can't parse `replicationFactor' value: `%v'. Value can be either positive integer or `inherit' (if Kafka version 2.4+) for server defaults", replicationFactorStr)
		}
	}
	return replicationFactor, nil
}

type TopicRegistrationRespDto struct {
	Addresses         map[KafkaProtocol][]string       `json:"addresses"`
	Name              string                           `json:"name" example:"local.sampleTopic2.430d243ba9c74f25bf05f492a52804fd"`
	Classifier        Classifier                       `json:"classifier"`
	Namespace         string                           `json:"namespace" example:"local"`
	ExternallyManaged bool                             `json:"externallyManaged"`
	Instance          string                           `json:"instance" example:"localkafka"`
	CACert            string                           `json:"caCert,omitempty" example:" "`
	Credentials       map[KafkaRole][]KafkaCredentials `json:"credential,omitempty"`
	InstanceRef       *KafkaInstance                   `json:"-"`
	RequestedSettings *TopicSettings                   `json:"requestedSettings"`
	ActualSettings    *TopicSettings                   `json:"actualSettings"`
	Template          string                           `json:"template,omitempty" example:"2"`
	Versioned         bool                             `json:"versioned,omitempty" example:"false"`
}

type TopicSearchRequest struct {
	Classifier           Classifier `json:"classifier"`
	Topic                string     `json:"topic" example:"<exact topic name in kafka>"`
	Namespace            string     `json:"namespace" example:"<namespace to which topics belong>"`
	Instance             string     `json:"instance" example:"<id of kafka instance where topics are created>"`
	Template             int        `json:"template"`
	LeaveRealTopicIntact bool       `json:"leaveRealTopicIntact"`
	Versioned            *bool      `json:"versioned"`
}

func (searchReq TopicSearchRequest) IsEmpty() bool {
	return searchReq.Classifier.IsEmpty() &&
		searchReq.Topic == "" &&
		searchReq.Namespace == "" &&
		searchReq.Instance == "" &&
		searchReq.Template == 0
}

type TopicDeletionResp struct {
	DeletedSuccessfully []TopicRegistrationRespDto `json:"deletedSuccessfully"`
	FailedToDelete      []TopicDeletionError       `json:"failedToDelete"`
}

type TopicDeletionError struct {
	Topic   *TopicRegistrationRespDto `json:"topic"`
	Message string                    `json:"message" example:"<error message>"`
}

type SyncTenantsResp struct {
	Tenant Tenant                      `json:"tenant"`
	Topics []*TopicRegistrationRespDto `json:"topics"`
}
