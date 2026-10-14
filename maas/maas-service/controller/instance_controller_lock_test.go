package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	mock_instance "github.com/netcracker/qubership-maas/service/instance/mock"
	"github.com/stretchr/testify/assert"
)

const (
	lockTestRabbitBody = `{"id": "rabbit-main", "apiUrl": "http://rabbit:15672/api", "amqpUrl": "amqp://rabbit:5672", "user": "u", "password": "p"}`
	lockTestKafkaBody  = `{"id": "kafka-main", "addresses": {"PLAINTEXT": ["kafka:9092"]}, "maasProtocol": "PLAINTEXT",
		"credentials": {"admin": [{"type": "SCRAM", "username": "a", "password": "plain:a"}], "client": [{"type": "SCRAM", "username": "c", "password": "plain:c"}]}}`
)

var operatorOwned = model.InstanceOwnership{ManagedByOperator: true, CRNamespace: "infra", CRName: "main"}

func TestInstanceController_RestLock(t *testing.T) {
	tests := []struct {
		name            string
		operatorEnabled bool
		existing        *model.InstanceOwnership
		wantStatus      int
	}{
		{name: "operator enabled, instance owned by CR", operatorEnabled: true, existing: &operatorOwned, wantStatus: http.StatusConflict},
		{name: "operator enabled, instance not owned", operatorEnabled: true, existing: &model.InstanceOwnership{}, wantStatus: http.StatusOK},
		{name: "operator enabled, instance not found", operatorEnabled: true, existing: nil, wantStatus: http.StatusOK},
		{name: "operator disabled, instance owned by CR", operatorEnabled: false, existing: &operatorOwned, wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, op := range []string{"update", "unregister"} {
				t.Run("rabbit "+op, func(t *testing.T) {
					mockCtrl := gomock.NewController(t)
					rabbit := mock_instance.NewMockRabbitInstanceService(mockCtrl)
					if tt.operatorEnabled {
						var existing *model.RabbitInstance
						if tt.existing != nil {
							existing = &model.RabbitInstance{Id: "rabbit-main", InstanceOwnership: *tt.existing}
						}
						rabbit.EXPECT().GetById(gomock.Any(), "rabbit-main").Return(existing, nil)
					}
					if tt.wantStatus == http.StatusOK {
						if op == "update" {
							rabbit.EXPECT().Update(gomock.Any(), gomock.Any()).Return(&model.RabbitInstance{}, nil)
						} else {
							rabbit.EXPECT().Unregister(gomock.Any(), "rabbit-main").Return(&model.RabbitInstance{}, nil)
						}
					}

					c := NewInstanceController(nil, rabbit, nil, OperatorSettings{Enabled: tt.operatorEnabled})
					handler, body := c.UpdateRabbitInstance, lockTestRabbitBody
					if op == "unregister" {
						handler, body = c.UnregisterRabbitInstance, `{"id": "rabbit-main"}`
					}
					assertLockResponse(t, handler, body, tt.wantStatus, "rabbit instance `rabbit-main'")
				})

				t.Run("kafka "+op, func(t *testing.T) {
					mockCtrl := gomock.NewController(t)
					kafka := mock_instance.NewMockKafkaInstanceService(mockCtrl)
					if tt.operatorEnabled {
						var existing *model.KafkaInstance
						if tt.existing != nil {
							existing = &model.KafkaInstance{Id: "kafka-main", InstanceOwnership: *tt.existing}
						}
						kafka.EXPECT().GetById(gomock.Any(), "kafka-main").Return(existing, nil)
					}
					if tt.wantStatus == http.StatusOK {
						if op == "update" {
							kafka.EXPECT().Update(gomock.Any(), gomock.Any()).Return(&model.KafkaInstance{}, nil)
						} else {
							kafka.EXPECT().Unregister(gomock.Any(), "kafka-main").Return(&model.KafkaInstance{}, nil)
						}
					}

					c := NewInstanceController(kafka, nil, nil, OperatorSettings{Enabled: tt.operatorEnabled})
					handler, body := c.UpdateKafkaInstance, lockTestKafkaBody
					if op == "unregister" {
						handler, body = c.UnregisterKafkaInstance, `{"id": "kafka-main"}`
					}
					assertLockResponse(t, handler, body, tt.wantStatus, "kafka instance `kafka-main'")
				})
			}
		})
	}
}

func assertLockResponse(t *testing.T, handler fiber.Handler, body string, wantStatus int, instanceRef string) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	app.Post("/instance", handler)

	resp, err := app.Test(httptest.NewRequest("POST", "/instance", strings.NewReader(body)))
	assert.NoError(t, err)
	assert.Equal(t, wantStatus, resp.StatusCode)

	if wantStatus == http.StatusConflict {
		respBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(respBody), instanceRef+" is managed by the MaaS operator through CR `main' in namespace `infra'")
		assert.Contains(t, string(respBody), "deletionPolicy: Orphan")
	}
}

func TestInstanceController_SetDefaultLock(t *testing.T) {
	tests := []struct {
		name       string
		operator   OperatorSettings
		wantStatus int
	}{
		{name: "operator enabled, param set", wantStatus: http.StatusConflict,
			operator: OperatorSettings{Enabled: true, DefaultKafkaInstance: "kafka-infra", DefaultRabbitInstance: "rabbit-infra"}},
		{name: "operator enabled, param empty", wantStatus: http.StatusOK,
			operator: OperatorSettings{Enabled: true}},
		{name: "operator disabled, param set", wantStatus: http.StatusOK,
			operator: OperatorSettings{DefaultKafkaInstance: "kafka-infra", DefaultRabbitInstance: "rabbit-infra"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Run("rabbit", func(t *testing.T) {
				rabbit := mock_instance.NewMockRabbitInstanceService(gomock.NewController(t))
				if tt.wantStatus == http.StatusOK {
					rabbit.EXPECT().SetDefault(gomock.Any(), gomock.Any()).Return(&model.RabbitInstance{}, nil)
				}
				c := NewInstanceController(nil, rabbit, nil, tt.operator)
				assertSetDefaultResponse(t, c.SetDefaultRabbitInstance, lockTestRabbitBody, tt.wantStatus,
					"default rabbit instance is selected by the DEFAULT_RABBIT_INSTANCE parameter (`rabbit-infra')")
			})

			t.Run("kafka", func(t *testing.T) {
				kafka := mock_instance.NewMockKafkaInstanceService(gomock.NewController(t))
				if tt.wantStatus == http.StatusOK {
					kafka.EXPECT().SetDefault(gomock.Any(), gomock.Any()).Return(&model.KafkaInstance{}, nil)
				}
				c := NewInstanceController(kafka, nil, nil, tt.operator)
				assertSetDefaultResponse(t, c.SetDefaultKafkaInstance, lockTestKafkaBody, tt.wantStatus,
					"default kafka instance is selected by the DEFAULT_KAFKA_INSTANCE parameter (`kafka-infra')")
			})
		})
	}
}

func assertSetDefaultResponse(t *testing.T, handler fiber.Handler, body string, wantStatus int, wantMessage string) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	app.Put("/instance/default", handler)

	resp, err := app.Test(httptest.NewRequest("PUT", "/instance/default", strings.NewReader(body)))
	assert.NoError(t, err)
	assert.Equal(t, wantStatus, resp.StatusCode)

	if wantStatus == http.StatusConflict {
		respBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Contains(t, string(respBody), wantMessage)
	}
}
