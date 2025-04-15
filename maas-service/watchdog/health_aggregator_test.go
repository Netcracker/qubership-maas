package watchdog

import (
	"github.com/go-errors/errors"
	"github.com/netcracker/qubership-maas/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestUtils_dbIsOkNoBrokers(t *testing.T) {
	ha := NewHealthAggregator(
		func() error { return nil },
		func() []BrokerStatus { return []BrokerStatus{} },
	)
	assert.Equal(t, UP, ha.Status().Status)
}

func TestUtils_dbFailOkNoBrokers(t *testing.T) {
	ha := NewHealthAggregator(
		func() error { return errors.New("oops") },
		func() []BrokerStatus { return []BrokerStatus{} },
	)
	assert.Equal(t, PROBLEM, ha.Status().Status)
}

func TestUtils_okAggergatedWithBrokers(t *testing.T) {
	ha := NewHealthAggregator(
		func() error { return nil },
		func() []BrokerStatus {
			return []BrokerStatus{
				{BrokerType: model.Kafka, Status: StatusDetailed{Status: UP}},
				{BrokerType: model.RabbitMQ, Status: StatusDetailed{Status: UP}},
			}
		},
	)
	assert.Equal(t, UP, ha.Status().Status)
}

func TestUtils_dbErrorAggergatedWithBrokers(t *testing.T) {
	ha := NewHealthAggregator(
		func() error { return errors.New("oops") },
		func() []BrokerStatus {
			return []BrokerStatus{
				{BrokerType: model.Kafka, Status: StatusDetailed{Status: UP}},
				{BrokerType: model.RabbitMQ, Status: StatusDetailed{Status: UP}},
			}
		},
	)
	assert.Equal(t, PROBLEM, ha.Status().Status)
}

func TestUtils_dbOkAggergatedWithBrokerInError(t *testing.T) {
	ha := NewHealthAggregator(
		func() error { return nil },
		func() []BrokerStatus {
			return []BrokerStatus{
				{BrokerType: model.Kafka, Status: StatusDetailed{Status: UP}},
				{BrokerType: model.Kafka, Status: StatusDetailed{Status: PROBLEM, Details: "failed"}},
				{BrokerType: model.Kafka, Status: StatusDetailed{Status: PROBLEM, Details: "failed"}},
				{BrokerType: model.RabbitMQ, Status: StatusDetailed{Status: UP}},
			}
		},
	)
	status := ha.Status()
	assert.Equal(t, PROBLEM, status.Status)
	assert.Equal(t, "failed, failed", status.Kafka.Details)
}
