package model

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	_assert "github.com/stretchr/testify/assert"
	"reflect"
	"testing"
	"time"
)

func TestMatchClassifiersWithNamespaceAndName(t *testing.T) {
	filter2 := Classifier{Name: "dictionary-changed", Namespace: "cloudbss-kube-dev1"}
	source2 := Classifier{Name: "metamodel-changed", Namespace: "cloudbss-kube-qa1"}
	if reflect.DeepEqual(source2, filter2) {
		t.Fatal("Different name and namespace returns true")
	}
}

func TestClassifierToStringEmpty(t *testing.T) {
	_assert.Equal(t, "{}", Classifier{}.String())
}

func TestParseVersion1(t *testing.T) {
	assert := _assert.New(t)
	version, err := ParseVersion("v1")
	assert.NoError(err)
	assert.Equal(Version("v1"), version)
}

func TestParseVersion2(t *testing.T) {
	assert := _assert.New(t)
	version, err := ParseVersion("1 ") // intentionally add extra space to check string trimming
	assert.NoError(err)
	assert.Equal(Version("v1"), version)
}

func TestEmptyVersion(t *testing.T) {
	assert := _assert.New(t)
	version, err := ParseVersion("")
	assert.NoError(err)
	assert.Equal(VersionEmpty, version)
}

func TestIncorrectVersion(t *testing.T) {
	assert := _assert.New(t)
	_, err := ParseVersion("abc")
	assert.Error(err)
}

func TestIncorrectVersion2(t *testing.T) {
	assert := _assert.New(t)
	_, err := ParseVersion("v1ff")
	assert.Error(err)
}

func TestResolveCreateReq(t *testing.T) {
	assert := _assert.New(t)

	topicReqDto := TopicRegistrationReqDto{
		Classifier: Classifier{
			Name:      "test-name",
			Namespace: "test-namespace",
			TenantId:  "test-tenant",
		},
		Name: "{{namespace}}_{{name}}_{{tenantId}}",
	}

	topic, err := topicReqDto.BuildTopicRegFromReq(nil)
	assert.NoError(err)
	topicJson, err := json.Marshal(topicReqDto)
	assert.NoError(err)
	assert.Equal(string(topicJson), topic.CreateReq)
}

func TestBgStatus_GetAllVersions(t *testing.T) {
	bgStatus := BgStatus{
		Id:         1,
		Namespace:  "namespace",
		Timestamp:  time.Time{},
		Active:     "v2",
		Legacy:     "v1",
		Candidates: []string{"v3", "v4"},
	}

	stringsArray := bgStatus.GetAllVersions()
	assert.Equal(t, 4, len(stringsArray))
}

func TestCpMessageDto_ConvertToBgStatus(t *testing.T) {
	cpMessageDto := CpMessageDto{
		CpDeploymentVersion{
			Version: "v1",
			Stage:   "LEGACY",
		},
		CpDeploymentVersion{
			Version: "v2",
			Stage:   "ACTIVE",
		},
		CpDeploymentVersion{
			Version: "v3",
			Stage:   "CANDIDATE",
		},
	}

	bgStatus, err := cpMessageDto.ConvertToBgStatus()
	assert.NoError(t, err)

	assert.Equal(t, "v2", bgStatus.Active)
	assert.Equal(t, "v1", bgStatus.Legacy)
}

func TestAccount_Marshaling(t *testing.T) {
	acc := Account{
		Username:         "scott",
		Roles:            []RoleName{ManagerRole},
		Salt:             "some-salt",
		Password:         "tiger",
		Namespace:        "core-dev",
		DomainNamespaces: nil,
	}

	assert.Equal(t, "{scott [manager] *** *** core-dev []}", fmt.Sprintf("%v", acc))

	accJson, err := json.Marshal(acc)
	assert.NoError(t, err)
	// check that json doesn't contain original password and salt
	assert.NotContains(t, accJson, string(acc.Salt))
	assert.NotContains(t, accJson, string(acc.Password))
}

func TestAccount_HasRoles(t *testing.T) {
	acc := Account{
		Roles: []RoleName{ManagerRole, AgentRole},
	}

	assert.True(t, acc.HasRoles(AgentRole))
	assert.False(t, acc.HasRoles("non-existing"))
	assert.True(t, acc.HasRoles(ManagerRole))

	assert.False(t, (&Account{}).HasRoles(AgentRole))
}

func TestAccount_HasAccessToNamespace(t *testing.T) {
	acc := Account{
		Namespace: "core-dev",
	}

	assert.False(t, acc.HasAccessToNamespace("other"))
	assert.True(t, acc.HasAccessToNamespace("core-dev"))

	acc = Account{
		Namespace: ManagerAccountNamespace,
	}
	assert.True(t, acc.HasAccessToNamespace("core-dev"))
	assert.True(t, acc.HasAccessToNamespace("other"))

}
