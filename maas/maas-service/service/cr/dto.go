package cr

const (
	CustomResourceStatusNotStarted CustomResourceWaitStatus = "NOT_STARTED"
	CustomResourceStatusInProgress CustomResourceWaitStatus = "IN_PROGRESS"
	CustomResourceStatusCompleted  CustomResourceWaitStatus = "COMPLETED"
	CustomResourceStatusFailed     CustomResourceWaitStatus = "FAILED"
	CustomResourceStatusTerminated CustomResourceWaitStatus = "TERMINATED"

	CustomResourceTypeValidated            CustomResourceType = "Validated"
	CustomResourceTypeDependenciesResolved CustomResourceType = "DependenciesResolved"
	CustomResourceTypeCreated              CustomResourceType = "Created"
)

type CustomResourceRequest struct {
	ApiVersion string                         `yaml:"apiVersion" validate:"required,eq_ignore_case=core.netcracker.com/v1"`
	Kind       string                         `yaml:"kind" validate:"required,eq_ignore_case=maas"`
	SubKind    string                         `yaml:"subKind" validate:"required,oneof=Topic TopicTemplate LazyTopic TenantTopic VHost"`
	Metadata   *CustomResourceMetadataRequest `yaml:"metadata" validate:"required"`
	Spec       *CustomResourceSpecRequest     `yaml:"spec" validate:"required"`
}

type CustomResourceMetadataRequest struct {
	Name      string `yaml:"name" validate:"required"`
	Namespace string `yaml:"namespace" validate:"required,lte=63"`
}

type CustomResourceSpecRequest map[string]any

type CustomResourceResponse struct {
	Status     CustomResourceWaitStatus   `json:"status" validate:"required,oneof=NOT_STARTED IN_PROGRESS COMPLETED FAILED TERMINATED"`
	TrackingId int64                      `json:"trackingId,omitempty"`
	Conditions []CustomResourceConditions `json:"conditions"`
}

type CustomResourceConditions struct {
	Type    CustomResourceType       `json:"type"`
	State   CustomResourceWaitStatus `json:"state"`
	Reason  string                   `json:"reason,omitempty"`
	Message string                   `json:"message,omitempty"`
}

var OkCRResponse = CustomResourceResponse{
	Status: CustomResourceStatusCompleted,
	Conditions: []CustomResourceConditions{
		{
			Type:  CustomResourceTypeValidated,
			State: CustomResourceStatusCompleted,
		},
		{
			Type:  CustomResourceTypeDependenciesResolved,
			State: CustomResourceStatusCompleted,
		},
		{
			Type:  CustomResourceTypeCreated,
			State: CustomResourceStatusCompleted,
		},
	},
}
