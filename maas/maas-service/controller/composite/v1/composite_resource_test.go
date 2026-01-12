package v1

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/controller"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/composite"
	"github.com/stretchr/testify/assert"
)

func TestRegistrationController_Create(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Post("/test", controller.WithJson[RegistrationRequest](c.Create))

	reqNoBaseline := httptest.NewRequest("POST", "/test", strings.NewReader(`
{
  "id": "test-baseline",
  "namespaces": [
    "first-namespace",
    "second-namespace"
  ]
}
`))

	resp, err := app.Test(reqNoBaseline, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(responseBody), "'namespaces' array MUST contain namespace from 'id' param")

	reqNamespaceToLong := httptest.NewRequest("POST", "/test", strings.NewReader(`
{
  "id": "test-baseline",
  "namespaces": [
    "first-namespace",
    "abcd012345678901234567890123456789012345678901234567890123456789"
  ]
}
`))

	resp, err = app.Test(reqNamespaceToLong, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(responseBody), "namespaces[1] length must be less than or equal to '63'")

	registrationService.EXPECT().Upsert(gomock.Any(), &composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b", "c"}})
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b", "c"]
			}
		`))

	resp, err = app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Empty(t, responseBody)
}

func TestRegistrationController_DeleteById(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Delete("/test/:id", c.DeleteById)

	req := httptest.NewRequest("DELETE", "/test/test-baseline", nil)

	registrationService.EXPECT().Destroy(gomock.Any(), "test-baseline")

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Empty(t, responseBody)

	registrationService.EXPECT().Destroy(gomock.Any(), "test-baseline").Return(msg.NotFound)

	resp, err = app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

func TestRegistrationController_GetAll(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Get("/test/", c.GetAll)

	req := httptest.NewRequest("GET", "/test", nil)
	registrationService.EXPECT().List(gomock.Any()).Return([]composite.CompositeRegistration{}, nil).Times(1)
	{
		resp, err := app.Test(req, 100)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		responseBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "[]", string(responseBody))
	}

	registrationService.EXPECT().List(gomock.Any()).Return([]composite.CompositeRegistration{
		{
			Id:         "a",
			Namespaces: []string{"a", "b"},
		}}, nil).Times(1)

	{
		resp, err := app.Test(req, 100)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		responseBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.NotEmpty(t, responseBody)

		var registrationResponse []RegistrationResponse
		assert.NoError(t, json.Unmarshal(responseBody, &registrationResponse))
		assert.Len(t, registrationResponse, 1)
		assert.Equal(t, RegistrationResponse{
			Id:         "a",
			Namespaces: []string{"a", "b"},
		}, registrationResponse[0])
	}
}

func TestRegistrationController_GetById(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Get("/test/:id", c.GetById)

	req := httptest.NewRequest("GET", "/test/a", nil)

	registrationService.EXPECT().GetByBaseline(gomock.Any(), gomock.Eq("a")).Return(
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}, nil)

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBody)

	var registrationResponse RegistrationResponse
	assert.NoError(t, json.Unmarshal(responseBody, &registrationResponse))
	assert.Equal(t, RegistrationResponse{Id: "a", Namespaces: []string{"a", "b"}}, registrationResponse)

	// test to request non existing domain
	registrationService.EXPECT().GetByBaseline(gomock.Any(), "non-existing").Return(nil, nil)
	req2 := httptest.NewRequest("GET", "/test/non-existing", nil)

	resp, err = app.Test(req2, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBody)
}
