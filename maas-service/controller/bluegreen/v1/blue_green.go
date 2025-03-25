package v1

import "maas/maas-service/service/bg2/domain"

type BGStateOperation struct {
	BGState *domain.BGState `json:"BGState"`
}

type ProcessStatus string

const (
	ProcessStatusNotStarted ProcessStatus = "not started"
	ProcessStatusInProgress ProcessStatus = "in progress"
	ProcessStatusFailed     ProcessStatus = "failed"
	ProcessStatusCompleted  ProcessStatus = "completed"
)

type SyncResponse struct {
	Status           ProcessStatus `json:"status,omitempty"`
	Message          string        `json:"message,omitempty"` // mandatory field
	OperationDetails string        `json:"operationDetails,omitempty"`
}
