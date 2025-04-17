package model

import (
	"database/sql"
	"fmt"
	"github.com/netcracker/qubership-maas/utils"
	"strings"
)

const ALTERNATE_EXCHANGE_SUFFIX = "ae"

type RabbitInstance struct {
	Id       string `pg:",pk" json:"id" example:"fd02de58-da04-4842-a258-37ebe4e5ac1e"`
	ApiUrl   string `pg:"api_url,notnull" json:"apiUrl" validate:"removeTrailingSlashes,http" example:"http://127.0.0.1:15672/api"`
	AmqpUrl  string `pg:"amqp_url,notnull" json:"amqpUrl" validate:"removeTrailingSlashes,amqp" example:"amqp://127.0.0.1:5672"`
	User     string `pg:",notnull" json:"user" example:"guest"`
	Password string `gorm:"column:password_enc;serializer:aesb64" pg:"password_enc,notnull" json:"password" fmt:"obfuscate" example:"guest"`
	Default  bool   `gorm:"column:is_default" pg:"is_default" json:"default" example:"true"`
}

func (r RabbitInstance) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(r, state, verb)
}

func (i *RabbitInstance) GetId() string {
	return i.Id
}

func (i *RabbitInstance) SetId(id string) {
	i.Id = id
}

func (i *RabbitInstance) IsDefault() bool {
	return i.Default
}

func (i *RabbitInstance) SetDefault(val bool) {
	i.Default = val
}

func (i *RabbitInstance) FillDefaults() {
	i.ApiUrl = strings.TrimRight(i.ApiUrl, "/")
	if !strings.HasSuffix(i.ApiUrl, "/api") {
		i.ApiUrl = i.ApiUrl + "/api"
	}
}

func (i *RabbitInstance) BrokerType() MessageBrokerType {
	return RabbitMQ
}

type InstanceDesignatorRabbit struct {
	Id                int     `json:"-"`
	Namespace         string  `json:"namespace"`
	DefaultInstanceId *string `json:"defaultInstance"`

	InstanceSelectors []*InstanceSelectorRabbit `gorm:"foreignKey:InstanceDesignatorId" json:"selectors"`
	DefaultInstance   *RabbitInstance           `json:"-"`
}

func (*InstanceDesignatorRabbit) TableName() string {
	return "instance_designators_rabbit"
}

func (designator *InstanceDesignatorRabbit) GetInstanceSelectors() []InstanceSelector {
	selectors := make([]InstanceSelector, len(designator.InstanceSelectors))
	for i, selector := range designator.InstanceSelectors {
		selectors[i] = selector
	}
	return selectors
}

type InstanceSelectorRabbit struct {
	Id                   int             `json:"-"`
	InstanceDesignatorId int             `json:"-"`
	ClassifierMatch      ClassifierMatch `gorm:"serializer:json" json:"classifierMatch"`
	InstanceId           string          `json:"instance"`

	Instance *RabbitInstance `json:"-"`
}

func (*InstanceSelectorRabbit) TableName() string {
	return "instance_designators_rabbit_selectors"
}

func (selector *InstanceSelectorRabbit) GetClassifierMatch() ClassifierMatch {
	return selector.ClassifierMatch
}

func (selector *InstanceSelectorRabbit) GetInstance() interface{} {
	return selector.Instance
}

type VHostRegistration struct {
	Id         int    `json:"-"`
	Vhost      string `json:"vhost"`
	User       string `json:"user"`
	Password   string `json:"password,omitempty" fmt:"obfuscate"`
	Namespace  string `json:"namespace"`
	InstanceId string `gorm:"column:instance" json:"instanceId,omitempty"`
	Classifier string `json:"classifier"`
}

func (VHostRegistration) TableName() string {
	return "rabbit_vhosts"
}

func (v VHostRegistration) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(v, state, verb)
}

func (reg *VHostRegistration) GetName() string {
	c, _ := ConvertToClassifier(reg.Classifier)
	return c.Name
}

func (reg *VHostRegistration) GetTenantId() string {
	c, _ := ConvertToClassifier(reg.Classifier)
	return c.TenantId
}

// todo changes for vault
func (reg *VHostRegistration) GetDecodedPassword() string {
	return strings.TrimPrefix(reg.Password, "plain:")
}

type MsConfig struct {
	Id               int                     `json:"-"`
	Namespace        string                  `json:"namespace"`
	MsName           string                  `json:"ms_name"`
	VhostID          int                     `json:"vhost_id"`
	CandidateVersion string                  `json:"candidate_version"`
	ActualVersion    string                  `json:"actual_version"`
	Config           ApplyRabbitConfigReqDto `gorm:"serializer:json"  json:"config"`

	Vhost    *VHostRegistration
	Entities []*RabbitVersionedEntity
}

func (MsConfig) TableName() string {
	return "rabbit_ms_configs"
}

type RabbitVersionedEntity struct {
	Id                 int                    `json:"-"`
	MsConfigId         int                    `json:"-"`
	EntityType         string                 `json:"entity_type"`
	EntityName         string                 `json:"entity_name,omitempty"`
	BindingSource      string                 `json:"binding_source,omitempty"`
	BindingDestination string                 `json:"binding_destination,omitempty"`
	ClientEntity       map[string]interface{} `gorm:"serializer:json;default:NULL" json:"client_entity,omitempty"`
	RabbitEntity       map[string]interface{} `gorm:"serializer:json;default:NULL" json:"rabbit_entity,omitempty"`

	MsConfig *MsConfig `json:"-"`
}

func (RabbitVersionedEntity) TableName() string {
	return "rabbit_versioned_entities"
}

type RabbitResult struct {
	VHostResp        VHostRegistrationResponse `json:"vhost"`
	VHostReg         VHostRegistration         `json:"-"`
	Entities         *RabbitEntities           `json:"entities,omitempty"`
	Policies         []interface{}             `json:"policies,omitempty"`
	UpdateStatus     []UpdateStatus            `json:"updateStatus,omitempty"`
	*RabbitDeletions `json:"deletions,omitempty"`
	Error            error `json:"-"`
}

type RabbitResultPerConfig struct {
	ServiceName string
	Result      RabbitResult
}

type RabbitEntityType int

const (
	ExchangeType RabbitEntityType = iota
	QueueType
	BindingType
)

func (d RabbitEntityType) String() string {
	return [...]string{"exchange", "queue", "binding"}[d]
}

type Exchange map[string]interface{}
type Queue map[string]interface{}

type RabbitEntities struct {
	//todo: rabbit entites should be refactored to specific type []map[string]interface{}
	Exchanges []interface{} `json:"exchanges,omitempty"`
	Queues    []interface{} `json:"queues,omitempty"`
	Bindings  []interface{} `json:"bindings,omitempty"`
}

type RabbitPolicies struct {
	Policies []interface{} `json:"policies,omitempty"`
}

type RabbitDeletions struct {
	RabbitEntities
	RabbitPolicies
}

type VhostAndVersion struct {
	Vhost   VHostRegistration
	Version string
}

type Shovel struct {
	Value ShovelValue `json:"value"`
	Vhost string      `json:"vhost"`
	Name  string      `json:"name"`
}
type ShovelValue struct {
	SrcProtocol  string `json:"src-protocol"`
	SrcUri       string `json:"src-uri"`
	SrcQueue     string `json:"src-queue"`
	DestProtocol string `json:"dest-protocol"`
	DestUri      string `json:"dest-uri"`
	DestQueue    string `json:"dest-queue,omitempty"`
	DestExchange string `json:"dest-exchange,omitempty"`
}

func (e RabbitEntities) IsNotEmpty() bool {
	return len(e.Exchanges) > 0 || len(e.Queues) > 0 || len(e.Bindings) > 0
}

type EntityWithName struct {
	Name string `json:"name"`
}

type EntityWithSourceAndDestination struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type UpdateStatus struct {
	Entity interface{} `json:"entity,omitempty"`
	Status string      `json:"status,omitempty"`
	Reason string      `json:"reason,omitempty"`
}

type LazyBindingDto struct {
	Vhost           string                 `json:"vhost,omitempty" example:"maas.core-dev.test"`
	Entity          map[string]interface{} `json:"entity,omitempty" gorm:"serializer:json"`
	ExchangeVersion string                 `json:"exchangeVersion,omitempty" example:"v1"`
	QueueVersion    string                 `json:"queueVersion,omitempty" example:"v1"`
}

type RabbitEntity struct {
	Id                 int                    `json:"-"`
	VhostId            int                    `json:"-"`
	Namespace          string                 `json:"namespace"`
	Classifier         string                 `json:"classifier"`
	EntityType         string                 `json:"entity_type"`
	EntityName         sql.NullString         `json:"entity_name,omitempty"`
	BindingSource      sql.NullString         `json:"binding_source,omitempty"`
	BindingDestination sql.NullString         `json:"binding_destination,omitempty"`
	ClientEntity       map[string]interface{} `gorm:"serializer:json;default:NULL" json:"client_entity,omitempty"`
	RabbitEntity       map[string]interface{} `gorm:"serializer:json;default:NULL" json:"rabbit_entity,omitempty"`
}

func (RabbitEntity) TableName() string {
	return "rabbit_entities"
}

func NewRabbitEntity(clientEntity interface{}, createdEntity *map[string]interface{}, entType RabbitEntityType, vhost VHostRegistration, classifier Classifier) (*RabbitEntity, error) {
	var name, source, destination string
	var err error
	if entType == ExchangeType || entType == QueueType {
		name, err = utils.ExtractName(clientEntity)
		if err != nil {
			return nil, err
		}
	} else if entType == BindingType {
		source, destination, err = utils.ExtractSourceAndDestination(clientEntity)
	}

	entMap := clientEntity.(map[string]interface{})

	rabbitEntity := RabbitEntity{
		VhostId:    vhost.Id,
		Namespace:  vhost.Namespace,
		Classifier: classifier.String(),
		EntityType: entType.String(),
		EntityName: sql.NullString{
			String: name,
			Valid:  name != "",
		},
		BindingSource: sql.NullString{
			String: source,
			Valid:  source != "",
		},
		BindingDestination: sql.NullString{
			String: destination,
			Valid:  destination != "",
		},
		ClientEntity: entMap,
	}

	if createdEntity != nil {
		rabbitEntity.RabbitEntity = *createdEntity
	}

	return &rabbitEntity, nil
}
