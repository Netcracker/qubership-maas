package model

import (
	"encoding/json"
	"fmt"
	"github.com/lib/pq"
	"github.com/netcracker/qubership-maas/utils"
	"golang.org/x/exp/slices"
	"regexp"
	"strings"
)

type RoleName string

const (
	ManagerAccountNamespace = "_GLOBAL"

	ManagerRole    RoleName = "manager"
	AgentRole      RoleName = "agent"
	BgOperatorRole RoleName = "bgoperator"
	AnonymousRole  RoleName = "anonymous"
)

type Account struct {
	Username         string             `json:"username" example:"manager"`
	Roles            []RoleName         `gorm:"serializer:json" json:"manager" example:"client"`
	Salt             utils.SecretString `json:"salt" example:"***"`
	Password         utils.SecretString `json:"password" example:"***"`
	Namespace        string             `json:"namespace" example:"_GLOBAL"`
	DomainNamespaces pq.StringArray     `pg:"domain_namespaces" gorm:"type:text[];->" example:"namespace1"`
}

func (acc *Account) IsAttachedTo(namespace string) bool {
	return slices.Contains(acc.DomainNamespaces, namespace)
}

func (acc *Account) HasRoles(expectedRoles ...RoleName) bool {
	for _, permittedRole := range acc.Roles {
		for _, expectedRole := range expectedRoles {
			if permittedRole == expectedRole {
				return true
			}
		}
	}
	return false
}

func (acc *Account) GetNamespaces() []string {
	return strings.Split(acc.Namespace, ",")
}

func (acc *Account) HasAccessToNamespace(namespace string) bool {
	if acc == nil {
		return false
	}
	if acc.Namespace == ManagerAccountNamespace {
		return true
	}
	for _, accountNamespace := range acc.GetNamespaces() {
		if namespace == accountNamespace {
			return true
		}
	}
	return false
}

type ResponseObject struct {
	ResponseBody *interface{}
	HttpCode     int
	Err          error
}

type MessageBrokerType string

const (
	Kafka    MessageBrokerType = "Kafka"
	RabbitMQ MessageBrokerType = "RabbitMQ"
)

type MessageBrokerInstance interface {
	GetId() string
	SetId(string)
	IsDefault() bool
	SetDefault(bool)
	FillDefaults()
	BrokerType() MessageBrokerType
}

type ClassifierMatch struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
	TenantId  string `json:"tenantId"`
	Instance  string `json:"instance"`
}

type SearchForm struct {
	Vhost      string
	User       string
	Namespace  string `json:"namespace" example:"cloudbss311-platform-core-support-dev3"`
	Instance   string
	Classifier Classifier
}

func (sf SearchForm) IsEmpty() bool {
	return sf.Classifier.IsEmpty() &&
		sf.Vhost == "" && sf.User == "" && sf.Namespace == "" && sf.Instance == ""
}

func ConvertToSearchForm(request string) (*SearchForm, error) {
	var data SearchForm
	if err := json.Unmarshal([]byte(request), &data); err != nil {
		return nil, err
	} else {
		return &data, nil
	}
}

type Namespace struct {
	Namespace string `json:"namespace" example:"<your namespace>"`
}

type Version string

const VersionEmpty = Version("")

func ParseVersion(version string) (Version, error) {
	version = strings.TrimSpace(version)
	if version == "" {
		return VersionEmpty, nil
	}

	pattern := regexp.MustCompile(`^v?(\d+)$`)
	groups := pattern.FindStringSubmatch(version)
	if groups == nil {
		return "", fmt.Errorf("incorrect version string format: `%s'. Expetected: %s", version, pattern.String())
	}

	return Version("v" + groups[1]), nil
}

type Tenant struct {
	Id                 int                    `json:"-"`
	Namespace          string                 `json:"namespace"`
	ExternalId         string                 `json:"externalId" example:"101"`
	TenantPresentation map[string]interface{} `gorm:"serializer:json" json:"tenantPresentation"`
}

type InstanceDesignator interface {
	GetInstanceSelectors() []InstanceSelector
}

type InstanceSelector interface {
	GetClassifierMatch() ClassifierMatch
	GetInstance() interface{}
}
