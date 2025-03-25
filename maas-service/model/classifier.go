package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/exp/slices"
	"maas/maas-service/msg"
	"strings"
)

type Classifier struct {
	Name      string `json:"name,omitempty" validate:"required"`
	Namespace string `json:"namespace,omitempty" validate:"required"`
	TenantId  string `json:"tenantId,omitempty"`
}

func (classifier Classifier) GetName() string {
	return classifier.Name
}
func (classifier Classifier) GetNamespace() string {
	return classifier.Namespace
}
func (classifier Classifier) GetTenantId() string {
	return classifier.TenantId
}

func NewClassifierFromReq(reqBody string) (Classifier, error) {
	reqBody = strings.TrimSpace(reqBody)
	var classifier Classifier
	if reqBody == "" {
		return classifier, errors.New("failed to parse classifier: classifier string is null or empty")
	}
	if err := json.Unmarshal([]byte(reqBody), &classifier); err != nil {
		return classifier, fmt.Errorf("error parse json to classifier: %w", err)
	}
	return classifier, nil
}

func (classifier Classifier) CheckAllowedNamespaces(allowedNamespaces ...string) error {
	if len(allowedNamespaces) == 0 {
		return fmt.Errorf("allowed namespaces should not be empty. classifier: %+v: %w", classifier, msg.BadRequest)
	}

	if !slices.Contains(allowedNamespaces, classifier.Namespace) {
		return fmt.Errorf("wrong classifier namespace '%+v': allowed namespaces: '[%s]': %w", classifier, strings.Join(allowedNamespaces, ", "), msg.AuthError)
	}

	return nil
}

func (classifier Classifier) ToJsonString() string {
	result, err := json.Marshal(&classifier)
	if err != nil {
		panic(err)
	}
	return string(result)
}

func (classifier Classifier) String() string {
	return classifier.ToJsonString()
}

func (classifier Classifier) IsEmpty() bool {
	return classifier.Name == "" && classifier.Namespace == "" && classifier.TenantId == ""
}

func ConvertToClassifier(request string) (Classifier, error) {
	var data Classifier
	if err := json.Unmarshal([]byte(request), &data); err == nil {
		return data, nil
	} else {
		return data, err
	}
}
