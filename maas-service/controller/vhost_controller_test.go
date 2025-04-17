package controller

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	mock_auth "github.com/netcracker/qubership-maas/service/auth/mock"
	mock_rabbit_service "github.com/netcracker/qubership-maas/service/rabbit_service/mock"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var vhostReq = `{
  "classifier": {
	"name": "public",
	"namespace": "test-namespace"
  }
}
`

var classifierReq = `
{
  "name": "public",
  "namespace": "test-namespace"
}
`

func TestVHostController_GetOrCreateVHost(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	authService := mock_auth.NewMockAuthService(mockCtrl)

	app := fiber.New()
	app.Use(ExtractRequestContext)
	app.Post("/getOrCreateVhost", NewVHostController(rabbitService, authService).GetOrCreateVHost)

	rabbitService.EXPECT().
		GetOrCreateVhost(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, &model.VHostRegistration{
			User:      "test-user",
			Password:  "test-password",
			Namespace: "test-namespace",
		}, nil).Times(2)
	rabbitService.EXPECT().GetConnectionUrl(gomock.Any(), gomock.Any()).
		Return("amqp://localhost:5432", nil).Times(2)
	rabbitService.EXPECT().GetApiUrl(gomock.Any(), gomock.Any()).
		Return("http://localhost:15672/api", nil).Times(1)

	req := httptest.NewRequest("POST", "/getOrCreateVhost", strings.NewReader(vhostReq))
	req.Header.Set(HeaderXNamespace, "test-namespace")
	resp, _ := app.Test(req)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	var vHostRegistration model.VHostRegistrationResponse
	assert.NoError(t, json.Unmarshal(body, &vHostRegistration))

	assert.Equal(t, "test-user", vHostRegistration.Username)
	assert.Equal(t, "test-password", vHostRegistration.Password)
	assert.Equal(t, "amqp://localhost:5432", vHostRegistration.Cnn)
	assert.Equal(t, "", vHostRegistration.ApiUrl)

	req = httptest.NewRequest("POST", "/getOrCreateVhost?extended=true", strings.NewReader(vhostReq))
	req.Header.Set(HeaderXNamespace, "test-namespace")
	resp, _ = app.Test(req)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(body, &vHostRegistration))

	assert.Equal(t, "test-user", vHostRegistration.Username)
	assert.Equal(t, "test-password", vHostRegistration.Password)
	assert.Equal(t, "amqp://localhost:5432", vHostRegistration.Cnn)
	assert.Equal(t, "http://localhost:15672/api", vHostRegistration.ApiUrl)
}

func TestVHostController_GetVHostAndConfigByClassifier(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	rabbitService := mock_rabbit_service.NewMockRabbitService(mockCtrl)
	authService := mock_auth.NewMockAuthService(mockCtrl)

	app := fiber.New()
	app.Use(ExtractRequestContext)
	app.Post("/getByClassifier", NewVHostController(rabbitService, authService).GetVHostAndConfigByClassifier)

	rabbitService.EXPECT().
		FindVhostByClassifier(gomock.Any(), gomock.Any()).
		Return(&model.VHostRegistration{
			User:      "test-user",
			Password:  "test-password",
			Namespace: "test-namespace",
		}, nil).Times(2)
	rabbitService.EXPECT().GetConfig(gomock.Any(), gomock.Any()).Times(2)
	rabbitService.EXPECT().GetConnectionUrl(gomock.Any(), gomock.Any()).
		Return("amqp://localhost:5432", nil).Times(2)
	rabbitService.EXPECT().GetApiUrl(gomock.Any(), gomock.Any()).
		Return("http://localhost:15672/api", nil).Times(1)

	req := httptest.NewRequest("POST", "/getByClassifier", strings.NewReader(classifierReq))
	req.Header.Set(HeaderXNamespace, "test-namespace")
	resp, _ := app.Test(req)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	var vhostAndConfigResult VhostAndConfigResult
	assert.NoError(t, json.Unmarshal(body, &vhostAndConfigResult))

	assert.Equal(t, "test-user", vhostAndConfigResult.VHost.Username)
	assert.Equal(t, "test-password", vhostAndConfigResult.VHost.Password)
	assert.Equal(t, "amqp://localhost:5432", vhostAndConfigResult.VHost.Cnn)
	assert.Equal(t, "", vhostAndConfigResult.VHost.ApiUrl)

	req = httptest.NewRequest("POST", "/getByClassifier?extended=true", strings.NewReader(classifierReq))
	req.Header.Set(HeaderXNamespace, "test-namespace")
	resp, _ = app.Test(req)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	body, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NoError(t, json.Unmarshal(body, &vhostAndConfigResult))

	assert.Equal(t, "test-user", vhostAndConfigResult.VHost.Username)
	assert.Equal(t, "test-password", vhostAndConfigResult.VHost.Password)
	assert.Equal(t, "amqp://localhost:5432", vhostAndConfigResult.VHost.Cnn)
	assert.Equal(t, "http://localhost:15672/api", vhostAndConfigResult.VHost.ApiUrl)
}
