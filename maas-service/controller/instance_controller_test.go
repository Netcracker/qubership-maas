package controller

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"io"
	"maas/maas-service/model"
	mock_instance "maas/maas-service/service/instance/mock"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInstanceController_RegisterNewKafkaInstance(t *testing.T) {
	addInstanceReqNoClientCred := `
{
  "id": "kafka-main",
  "addresses": {
    "PLAINTEXT": [
      "localhost:9092"
    ]
  },
  "credentials": {
    "admin": [
      {
        "password": "plain:admin",
        "type": "SCRAM",
        "username": "admin"
      }
    ]
  },
  "maasProtocol": "PLAINTEXT",
  "default": true
}
`

	addInstanceReqOk := `
{
  "id": "kafka-main",
  "addresses": {
    "PLAINTEXT": [
      "localhost:9092"
    ]
  },
  "credentials": {
    "admin": [
      {
        "password": "plain:admin",
        "type": "SCRAM",
        "username": "admin"
      }
    ],
    "client": [
      {
        "password": "plain:admin",
        "type": "SCRAM",
        "username": "admin"
      }
    ]
  },
  "maasProtocol": "PLAINTEXT",
  "default": true
}
`
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	_, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()

	mockCtrl := gomock.NewController(t)
	kafkaInstanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)

	instanceController := NewInstanceController(kafkaInstanceService, nil, nil)
	app.Post("/kafka/instance", instanceController.RegisterNewKafkaInstance)

	req := httptest.NewRequest("POST", "/kafka/instance", strings.NewReader(addInstanceReqNoClientCred))
	req.SetBasicAuth(managerName, managerPassword)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	msg, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(msg), "\"credentials must have 'admin' and 'client' roles\"")

	kafkaInstanceService.EXPECT().
		Register(gomock.Any(), gomock.Any()).
		Return(&model.KafkaInstance{}, nil)
	req = httptest.NewRequest("POST", "/kafka/instance", strings.NewReader(addInstanceReqOk))
	req.SetBasicAuth(managerName, managerPassword)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
