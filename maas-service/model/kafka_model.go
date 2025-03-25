package model

import (
	"database/sql"
	"fmt"
	"github.com/lib/pq"
	"maas/maas-service/utils"
)

type KafkaRole string

const (
	Admin  KafkaRole = "admin"
	Client KafkaRole = "client"
)

const (
	StatusOk     = "ok"
	StatusAbsent = "absent"
)

type AuthType string

const (
	PlainAuth        AuthType = "plain"
	SslCertAuth      AuthType = "sslCert"
	SCRAMAuth        AuthType = "SCRAM"
	SslCertPlusPlain AuthType = "sslCert+plain"
	SslCertPlusSCRAM AuthType = "sslCert+SCRAM"
)

type KafkaCredentials struct {
	AuthType   AuthType `json:"type" validate:"kafkaAuth"`
	Username   string   `json:"username,omitempty"`
	Password   string   `json:"password,omitempty"`
	ClientKey  string   `json:"clientKey,omitempty"`
	ClientCert string   `json:"clientCert,omitempty"`
}

func (creds KafkaCredentials) GetAuthType() AuthType {
	return creds.AuthType
}

func (creds KafkaCredentials) GetBasicAuth() BasicAuth {
	return BasicAuth{Username: creds.Username, Password: []byte(creds.Password)}
}

func (creds KafkaCredentials) GetSslCertAuth() SslCert {
	return SslCert{ClientKey: creds.ClientKey, ClientCert: creds.ClientCert}
}

type BasicAuth struct {
	Username string `json:"username"`
	Password []byte `json:"password" fmt:"obfuscate"`
}

func (b BasicAuth) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(b, state, verb)
}

type SslCert struct {
	ClientKey  string `json:"clientKey" fmt:"obfuscate"`
	ClientCert string `json:"clientCert"`
}

func (s SslCert) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(s, state, verb)
}

type PasswordType string

const (
	Vault PasswordType = "vault"
	Plain PasswordType = "plain"
)

const (
	SyncStatusExists   = "exists"
	SyncStatusAdded    = "added"
	SyncStatusError    = "error"
	SyncStatusNotFound = "not_found"
)

type KafkaTopicSyncReport struct {
	Name       string     `json:"name" example:"maas.local.test-topic-0"`
	Classifier Classifier `json:"classifier"`
	Status     string     `json:"status" example:"added"`
	ErrMsg     string     `json:"errMsg,omitempty" example:"can not get topic info"`
}

type KafkaProtocol string

const (
	Plaintext     KafkaProtocol = "PLAINTEXT"
	SaslPlaintext KafkaProtocol = "SASL_PLAINTEXT"
	Ssl           KafkaProtocol = "SSL"
	SaslSsl       KafkaProtocol = "SASL_SSL"
)

type KafkaInstance struct {
	Id           string                           `pg:",pk"           json:"id" example:"cpq-kafka-maas-test"`
	Addresses    map[KafkaProtocol][]string       `gorm:"serializer:json" pg:"addresses"  json:"addresses" validate:"dive,keys,kafkaProtocol,endkeys"`
	Default      bool                             `gorm:"column:is_default" pg:"is_default"    json:"default" example:"true"`
	MaasProtocol KafkaProtocol                    `pg:"maas_protocol" json:"maasProtocol" validate:"kafkaProtocol" example:"SASL_PLAINTEXT"`
	CACert       string                           `pg:"ca_cert"       json:"caCert,omitempty"      fmt:"obfuscate" example:"MIIDPjCCAiYCCQCNmVmmEXs5XjANBgkqhkiG9w0BAQsFADBVMQswCQYDVQQGEwJYWDEVMBMGA1UEBwwMRGVmYXVsdCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMREwDwYDVQQDDAhsb2NhbGs4czAeFw0yMDA4MTgxMjUzMDFaFw0yMDExMjUxMjUzMDFaMG0xEDAOBgNVBAYTB1Vua25vd24xEDAOBgNVBAgTB1Vua25vd24xEDAOBgNVBAcTB1Vua25vd24xEDAOBgNVBAoTB1Vua25vd24xEDAOBgNVBAsTB1Vua25vd24xETAPBgNVBAMTCGxvY2FsazhzMIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAjcTaRTo7E7UI9jO9gtRAi5au57elqRX2YMj/OOGcwQdzP6JfMFZsKFNUoIoF8bJ51JXhbDxVgB+GHvEMmQ0jqGnMjSTsdxEQUCRTnINMAIAYLBKm5FGi5pJodZRzhNKoWhloRO9/2p2AYB+T39MxXFch3fwMdghVKbSqOCo0nsqCZwyB5CcZgLi69qifZPQAIFPUPDHG5Z6oGUjE/p+45RnOcAdCOgO0QllxO+fioCMPizRqIiim88UuZU7EjhaIwSTjOIohcPQStNU6vAp0ZGIgr8BhAZHiL8JRDto37ayo7ltDYtLg4Ojo3e9ue8Dwo5PSs+N6Od8Z//Xq6V8zwQIDAQABMA0GCSqGSIb3DQEBCwUAA4IBAQCDsV/jDojj4t977V5BMSTDeELEvNX8VMMtqAGpB1jtVNVXRLfG2SAcv6ZdOUbyuBagF8D0dsV5VvcqPw8YHNpHMKCTSdcida28rV2C31M+XRvvh90eoPtXfE60wo4Ky4UbKiJERiBIXMFLrg8PZ51PukT4fD0DioNpIxzRFb4VkypYv4srADX5shSvJN9Zxdj2EywR+S1k2F4TIDdOnWY9xGMftJz1fc58dMFMwGi7Evr+pR/w7yWDcvRgdgAYUpGaehmnYhzuw4XKWmVX1D6aVFRhonHaxN0rUemPZYSqjHp+lWOiUsYqatxB5MGQqj1/QV3XTAWglFbk1BB2stQg"`
	Credentials  map[KafkaRole][]KafkaCredentials `gorm:"column:credentials_enc;serializer:aesb64" pg:"credentials_enc"   json:"credentials,omitempty" fmt:"obfuscate" validate:"kafkaCredentialsRoles"`
}

func (i KafkaInstance) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(i, state, verb)
}

func (i *KafkaInstance) GetId() string {
	return i.Id
}

func (i *KafkaInstance) SetId(id string) {
	i.Id = id
}

func (i *KafkaInstance) IsDefault() bool {
	return i.Default
}

func (i *KafkaInstance) SetDefault(val bool) {
	i.Default = val
}

func (i *KafkaInstance) FillDefaults() {
	if len(i.MaasProtocol) == 0 {
		i.MaasProtocol = Plaintext
	}
}

func (i *KafkaInstance) BrokerType() MessageBrokerType {
	return Kafka
}

type InstanceDesignatorKafka struct {
	Id                int     `json:"-"`
	Namespace         string  `json:"namespace"`
	DefaultInstanceId *string `json:"defaultInstance"`

	InstanceSelectors []*InstanceSelectorKafka `gorm:"foreignKey:InstanceDesignatorId" json:"selectors"`
	DefaultInstance   *KafkaInstance           `json:"-"`
}

func (*InstanceDesignatorKafka) TableName() string {
	return "instance_designators_kafka"
}

func (designator *InstanceDesignatorKafka) GetInstanceSelectors() []InstanceSelector {
	selectors := make([]InstanceSelector, len(designator.InstanceSelectors))
	for i, selector := range designator.InstanceSelectors {
		selectors[i] = selector
	}
	return selectors
}

type InstanceSelectorKafka struct {
	Id                   int             `json:"-"`
	InstanceDesignatorId int             `json:"-"`
	ClassifierMatch      ClassifierMatch `gorm:"serializer:json" json:"classifierMatch"`
	InstanceId           string          `json:"instance"`

	Instance *KafkaInstance `json:"-"`
}

func (*InstanceSelectorKafka) TableName() string {
	return "instance_designators_kafka_selectors"
}

func (selector *InstanceSelectorKafka) GetClassifierMatch() ClassifierMatch {
	return selector.ClassifierMatch
}

func (selector *InstanceSelectorKafka) GetInstance() interface{} {
	return selector.Instance
}

type TopicRegistration struct {
	Id                uint        `gorm:"primaryKey"`
	Classifier        *Classifier `gorm:"serializer:json"`
	Topic             string
	Instance          string
	Namespace         string
	ExternallyManaged bool
	TopicSettings     `gorm:"embedded"`
	Template          sql.NullInt64 `gorm:"default:NULL"`
	Dirty             bool
	TenantId          sql.NullInt64  `gorm:"default:NULL"`
	CreateReq         string         `pg:"create_req" gorm:"default:{}"`
	InstanceRef       *KafkaInstance `gorm:"-"`
}

func (TopicRegistration) TableName() string {
	return "kafka_topics"
}

func (topic *TopicRegistration) SetSettings(settings *TopicSettings) {
	topic.NumPartitions = settings.NumPartitions
	topic.MinNumPartitions = settings.MinNumPartitions
	topic.ReplicationFactor = settings.ReplicationFactor
	topic.ReplicaAssignment = settings.ReplicaAssignment
	topic.Configs = settings.Configs
}

func (topic *TopicRegistration) GetSettings() *TopicSettings {
	return &TopicSettings{
		NumPartitions:     topic.NumPartitions,
		MinNumPartitions:  topic.MinNumPartitions,
		ReplicationFactor: topic.ReplicationFactor,
		ReplicaAssignment: topic.ReplicaAssignment,
		Configs:           topic.Configs,
	}
}

func (topic *TopicRegistration) Equals(anotherTopic *TopicRegistration) bool {
	if anotherTopic == nil {
		return false
	}
	if topic == anotherTopic {
		return true
	}
	if topic.Topic != anotherTopic.Topic || topic.Instance != anotherTopic.Instance {
		return false
	}
	return topic.SettingsAreEqual(anotherTopic)
}

func (topic *TopicRegistration) SettingsAreEqual(anotherTopic *TopicRegistration) bool {
	if topic.Template != anotherTopic.Template {
		return false
	}
	if topic.NumPartitions != anotherTopic.NumPartitions {
		if topic.NumPartitions == nil || *topic.NumPartitions == -1 {
			if anotherTopic.NumPartitions != nil && *anotherTopic.NumPartitions != -1 {
				return false
			}
		} else if anotherTopic.NumPartitions == nil || *topic.NumPartitions != *anotherTopic.NumPartitions {
			return false
		}
	}
	if topic.MinNumPartitions != anotherTopic.MinNumPartitions {
		if topic.MinNumPartitions == nil || *topic.MinNumPartitions == -1 {
			if anotherTopic.MinNumPartitions != nil && *anotherTopic.MinNumPartitions != -1 {
				return false
			}
		} else if anotherTopic.MinNumPartitions == nil || *topic.MinNumPartitions != *anotherTopic.MinNumPartitions {
			return false
		}
	}
	if topic.ReplicationFactor != anotherTopic.ReplicationFactor {
		if topic.ReplicationFactor == nil || *topic.ReplicationFactor == -1 {
			if anotherTopic.ReplicationFactor != nil && *anotherTopic.ReplicationFactor != -1 {
				return false
			}
		} else if anotherTopic.ReplicationFactor == nil || *topic.ReplicationFactor != *anotherTopic.ReplicationFactor {
			return false
		}
	}
	if len(topic.ReplicaAssignment) != len(anotherTopic.ReplicaAssignment) {
		return false
	}
	if len(topic.ReplicaAssignment) > 0 {
		for key, val := range topic.ReplicaAssignment {
			if anotherVal, found := anotherTopic.ReplicaAssignment[key]; !found || !utils.SlicesAreEqualInt32(val, anotherVal) {
				return false
			}
		}
	}
	if topic.Versioned != anotherTopic.Versioned {
		return false
	}
	return utils.StringMapsAreEqual(topic.Configs, anotherTopic.Configs)
}

func (topic *TopicRegistration) ToResponseDto() *TopicRegistrationRespDto {
	if topic.InstanceRef == nil {
		panic("trying to access TopicRegistration#KafkaInstance which is nil")
	}

	result := TopicRegistrationRespDto{
		Addresses:         topic.InstanceRef.Addresses,
		Name:              topic.Topic,
		Classifier:        *topic.Classifier, // FIXME select only one classifier from list
		Namespace:         topic.Namespace,
		Instance:          topic.Instance,
		InstanceRef:       topic.InstanceRef,
		CACert:            topic.InstanceRef.CACert,
		ExternallyManaged: topic.ExternallyManaged,
		RequestedSettings: &TopicSettings{
			NumPartitions:     topic.NumPartitions,
			MinNumPartitions:  topic.MinNumPartitions,
			ReplicationFactor: topic.ReplicationFactor,
			ReplicaAssignment: topic.ReplicaAssignment,
			Configs:           topic.Configs,
		},
		ActualSettings: &TopicSettings{},
		Versioned:      topic.Versioned,
	}
	clientCredentials := topic.InstanceRef.Credentials[Client]
	if len(clientCredentials) > 0 {
		result.Credentials = map[KafkaRole][]KafkaCredentials{Client: clientCredentials}
	}
	return &result
}

type TopicSettings struct {
	NumPartitions     *int32             `json:"numPartitions,omitempty" example:"1"`
	MinNumPartitions  *int32             `json:"minNumPartitions,omitempty" example:"1"`
	ReplicationFactor *int16             `json:"replicationFactor,omitempty" example:"1"`
	ReplicaAssignment map[int32][]int32  `gorm:"serializer:json" json:"replicaAssignment,omitempty"`
	Configs           map[string]*string `gorm:"serializer:json" json:"configs,omitempty"`
	Versioned         bool               `json:"versioned,omitempty" example:"true"`
}

func (settings *TopicSettings) String() string {
	var num int32
	var repl int16
	var minNumPartitions int32
	if settings.NumPartitions != nil {
		num = *settings.NumPartitions
	}
	if settings.ReplicationFactor != nil {
		repl = *settings.ReplicationFactor
	}
	if settings.MinNumPartitions != nil {
		minNumPartitions = *settings.MinNumPartitions
	}

	return fmt.Sprintf("NumPartitions= %v, ReplicationFactor= %v, ReplicaAssignment= %v, Configs= %+v, MinNumPartitions= %d, Versioned= %t",
		num, repl, settings.ReplicaAssignment, settings.Configs, minNumPartitions, settings.Versioned)
}

func (settings *TopicSettings) Equals(anotherSettings *TopicSettings) bool {
	if settings.ReadOnlySettingsAreEqual(anotherSettings) == false {
		return false
	}
	return settings.ConfigsAreEqual(anotherSettings)
}

func (settings *TopicSettings) ActualNumPartitions() int32 {
	if !IsEmpty(settings.MinNumPartitions) {
		return *settings.MinNumPartitions
	}
	if !IsEmpty(settings.NumPartitions) {
		return *settings.NumPartitions
	}
	return 1
}

func (settings *TopicSettings) ConfigsAreEqual(anotherSettings *TopicSettings) bool {
	return utils.StringMapsAreEqual(settings.Configs, anotherSettings.Configs)
}

func (settings *TopicSettings) ReadOnlySettingsAreEqual(anotherSettings *TopicSettings) bool {
	if anotherSettings == nil {
		return false
	}
	if settings == anotherSettings {
		return true
	}
	if settings.NumPartitions != anotherSettings.NumPartitions {
		if settings.NumPartitions == nil || *settings.NumPartitions == -1 {
			if anotherSettings.NumPartitions != nil && *anotherSettings.NumPartitions != -1 {
				return false
			}
		} else if anotherSettings.NumPartitions == nil || *settings.NumPartitions != *anotherSettings.NumPartitions {
			return false
		}
	}
	if settings.MinNumPartitions != anotherSettings.MinNumPartitions {
		if settings.MinNumPartitions == nil || *settings.MinNumPartitions == -1 {
			if anotherSettings.MinNumPartitions != nil && *anotherSettings.MinNumPartitions != -1 {
				return false
			}
		} else if anotherSettings.MinNumPartitions == nil || *settings.MinNumPartitions != *anotherSettings.MinNumPartitions {
			return false
		}
	}
	if settings.ReplicationFactor != anotherSettings.ReplicationFactor {
		if settings.ReplicationFactor == nil || *settings.ReplicationFactor == -1 {
			if anotherSettings.ReplicationFactor != nil && *anotherSettings.ReplicationFactor != -1 {
				return false
			}
		} else if anotherSettings.ReplicationFactor == nil || *settings.ReplicationFactor != *anotherSettings.ReplicationFactor {
			return false
		}
	}
	if len(settings.ReplicaAssignment) != len(anotherSettings.ReplicaAssignment) {
		return false
	}
	if len(settings.ReplicaAssignment) > 0 {
		for key, val := range settings.ReplicaAssignment {
			if anotherVal, found := anotherSettings.ReplicaAssignment[key]; !found || !utils.SlicesAreEqualInt32(val, anotherVal) {
				return false
			}
		}
	}
	return true
}

type TopicTemplate struct {
	Id        int    `json:"-" example:"topic ID"`
	Name      string `json:"name" example:"topic Name"`
	Namespace string `json:"namespace" example:"local"`
	TopicSettings
	DomainNamespaces pq.StringArray `pg:"domain_namespaces" gorm:"type:text[]" json:"-" example:"namespace1"`
}

func (TopicTemplate) TableName() string {
	return "kafka_topic_templates"
}

func (topicTemplate *TopicTemplate) GetSettings() *TopicSettings {
	return &TopicSettings{
		NumPartitions:     topicTemplate.NumPartitions,
		MinNumPartitions:  topicTemplate.MinNumPartitions,
		ReplicationFactor: topicTemplate.ReplicationFactor,
		ReplicaAssignment: topicTemplate.ReplicaAssignment,
		Configs:           topicTemplate.Configs,
	}
}

type TopicDefinition struct {
	// golang json.Marshal sorts map keys, so we can compare classifier jsons as pure strings
	Id                uint               `gorm:"primaryKey" json:"-"`
	Classifier        *Classifier        `gorm:"serializer:json"`
	Namespace         string             `json:"namespace"`
	Name              string             `json:"name,omitempty"`
	Instance          string             `json:"instance,omitempty"`
	NumPartitions     int                `gorm:"type:int" json:"numPartitions,omitempty"`
	MinNumPartitions  int                `gorm:"type:int" json:"minNumPartitions,omitempty"`
	ReplicationFactor int                `gorm:"type:int" json:"replicationFactor,omitempty"`
	ReplicaAssignment map[int32][]int32  `gorm:"serializer:json" json:"replicaAssignment,omitempty"`
	Configs           map[string]*string `gorm:"serializer:json" json:"configs,omitempty"`
	Template          string             `json:",omitempty"`
	Kind              string             `json:",omitempty"`
	Versioned         bool               `json:"versioned,omitempty" example:"true"`
}

func (*TopicDefinition) TableName() string {
	return "kafka_topic_definitions"
}

const (
	TopicDefinitionKindLazy   = "lazy"
	TopicDefinitionKindTenant = "tenant"
)

func (topicDefinition *TopicDefinition) MakeTopicDefinitionResponse() *TopicRegistrationReqDto {
	classifier, err := NewClassifierFromReq(topicDefinition.Classifier.ToJsonString())
	if err != nil {
		return nil
	}
	return &TopicRegistrationReqDto{
		Name:       topicDefinition.Name,
		Classifier: classifier,
		Instance:   topicDefinition.Instance,
		// TODO unify all NumPartition and ReplicationFactor conversions to database value and back
		NumPartitions:    zeroToNil(utils.Iff((topicDefinition.NumPartitions == 0) && (topicDefinition.MinNumPartitions == 0), 1, topicDefinition.NumPartitions)),
		MinNumPartitions: zeroToNil(topicDefinition.MinNumPartitions),
		// TODO unify all NumPartition and ReplicationFactor conversions to database value and back
		ReplicationFactor: utils.Iff(topicDefinition.ReplicationFactor == 0, 1, topicDefinition.ReplicationFactor),
		ReplicaAssignment: topicDefinition.ReplicaAssignment,
		Configs:           topicDefinition.Configs,
		Template:          topicDefinition.Template,
		Versioned:         topicDefinition.Versioned,
	}
}

type DiscrepancyReportItem struct {
	Name       string     `json:"name" example:"maas.test.81.my-awesome-test-topic"`
	Classifier Classifier `json:"classifier"`
	Status     string     `json:"status" example:"ok"`
}

func IsEmpty[T int8 | int16 | int32 | int64](val *T) bool {
	if val == nil || *val == -1 || *val == 0 {
		return true
	}
	return false
}

func zeroToNil[T int8 | int16 | int32 | int64 | int](val T) any {
	if val == 0 {
		return nil
	}
	return val
}
