package watchdog

import (
	"maas/maas-service/model"
)

type HealthAggregator struct {
	dbIsAvailable   func() error
	instancesStatus func() []BrokerStatus
}

type AggregatedStatus struct {
	Status   Status          `json:"status,omitempty"`
	Postgres StatusDetailed  `json:"postgres,omitempty"`
	Kafka    *StatusDetailed `json:"kafka,omitempty"`
	Rabbit   *StatusDetailed `json:"rabbit,omitempty"`
}

func NewHealthAggregator(dbIsAvailable func() error, instancesStatus func() []BrokerStatus) *HealthAggregator {
	return &HealthAggregator{dbIsAvailable: dbIsAvailable, instancesStatus: instancesStatus}
}

func (a *HealthAggregator) Status() AggregatedStatus {
	result := AggregatedStatus{Status: UP}
	for _, e := range a.instancesStatus() {
		switch e.BrokerType {
		case model.RabbitMQ:
			result.Rabbit = aggregate(result.Rabbit, e)
		case model.Kafka:
			result.Kafka = aggregate(result.Kafka, e)
		}
	}

	if err := a.dbIsAvailable(); err == nil {
		result.Postgres.Status = UP
	} else {
		result.Postgres = StatusDetailed{PROBLEM, err.Error()}
	}

	// calculate overall status
	result.Status = result.Postgres.Status
	if result.Rabbit != nil {
		result.Status = max(result.Status, result.Rabbit.Status)
	}
	if result.Kafka != nil {
		result.Status = max(result.Status, result.Kafka.Status)
	}

	return result
}

func max(status ...Status) Status {
	result := UP
	for _, e := range status {
		if result.Int() < e.Int() {
			result = e
		}
	}
	return result
}

func aggregate(agg *StatusDetailed, appl BrokerStatus) *StatusDetailed {
	if agg == nil {
		return &appl.Status
	}

	result := *agg // make a copy

	if result.Status.Int() < appl.Status.Status.Int() {
		result.Status = appl.Status.Status
	}

	if appl.Status.Details != "" {
		if result.Details != "" {
			result.Details += ", "
		}
		result.Details += appl.Status.Details
	}

	return &result
}
