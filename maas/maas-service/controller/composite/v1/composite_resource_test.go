package v1

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/controller"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/composite"
	"github.com/stretchr/testify/assert"
)

func ptr[T any](v T) *T {
	return &v
}

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

	resp, err := app.Test(reqNoBaseline, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
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

	resp, err = app.Test(reqNamespaceToLong, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
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

	resp, err = app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Empty(t, responseBody)
}

func TestRegistrationController_Create_WithModifyIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Post("/test", controller.WithJson[RegistrationRequest](c.Create))

	registrationService.EXPECT().Upsert(gomock.Any(),
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(42))})

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b"],
			  "modifyIndex": 42
			}
		`))

	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	registrationService.EXPECT().Upsert(gomock.Any(),
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: nil})

	reqNoIndex := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b"]
			}
		`))

	resp, err = app.Test(reqNoIndex, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	registrationService.EXPECT().Upsert(gomock.Any(),
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(0))})

	reqZeroIndex := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b"],
			  "modifyIndex": 0
			}
		`))

	resp, err = app.Test(reqZeroIndex, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestRegistrationController_Create_NegativeModifyIndex(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Post("/test", controller.WithJson[RegistrationRequest](c.Create))

	// no Upsert expectation: request must be rejected before reaching the service
	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b"],
			  "modifyIndex": -1
			}
		`))

	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRegistrationController_ServiceErrorsArePropagated(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Post("/test", controller.WithJson[RegistrationRequest](c.Create))
	app.Get("/test", c.GetAll)
	app.Get("/test/:id", c.GetById)

	// stale modify index is reported by the dao as a bad request and must reach the client as 400
	registrationService.EXPECT().Upsert(gomock.Any(), gomock.Any()).
		Return(fmt.Errorf("new modify index '1' cannot be less than the current index '2': %w", msg.BadRequest))

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
			{
			  "id": "a",
			  "namespaces": ["a", "b"],
			  "modifyIndex": 1
			}
		`))

	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(responseBody), "new modify index '1' cannot be less than the current index '2'")

	registrationService.EXPECT().List(gomock.Any()).Return(nil, errors.New("db is down"))

	resp, err = app.Test(httptest.NewRequest("GET", "/test", nil), fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	registrationService.EXPECT().GetByBaseline(gomock.Any(), "a").Return(nil, errors.New("db is down"))

	resp, err = app.Test(httptest.NewRequest("GET", "/test/a", nil), fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestRegistrationRequest_ToCompositeRegistration(t *testing.T) {
	assert.Equal(t,
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}},
		RegistrationRequest{Id: "a", Namespaces: []string{"a", "b"}}.ToCompositeRegistration())

	assert.Equal(t,
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(7))},
		RegistrationRequest{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(7))}.ToCompositeRegistration())
}

func TestNewRegistrationResponse(t *testing.T) {
	assert.Equal(t,
		&RegistrationResponse{Id: "a", Namespaces: []string{"a", "b"}},
		NewRegistrationResponse(&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))

	// modify index is an input-only field, it is not exposed in responses
	assert.Equal(t,
		&RegistrationResponse{Id: "a", Namespaces: []string{"a", "b"}},
		NewRegistrationResponse(&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(7))}))
}

func TestRegistrationController_DeleteById(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	registrationService := NewMockRegistrationService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewRegistrationController(registrationService)

	app.Delete("/test/:id", c.DeleteById)

	req := httptest.NewRequest("DELETE", "/test/test-baseline", nil)

	registrationService.EXPECT().Destroy(gomock.Any(), "test-baseline")

	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Empty(t, responseBody)

	registrationService.EXPECT().Destroy(gomock.Any(), "test-baseline").Return(msg.NotFound)

	resp, err = app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
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
		resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
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
		resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
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

	registrationService.EXPECT().List(gomock.Any()).Return([]composite.CompositeRegistration{
		{
			Id:          "a",
			Namespaces:  []string{"a", "b"},
			ModifyIndex: ptr(uint64(13)),
		},
		{
			Id:         "d",
			Namespaces: []string{"d"},
		}}, nil).Times(1)

	{
		resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		responseBody, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		// modify index is not exposed in the response
		assert.NotContains(t, string(responseBody), "modifyIndex")

		var registrationResponse []RegistrationResponse
		assert.NoError(t, json.Unmarshal(responseBody, &registrationResponse))
		assert.Equal(t, []RegistrationResponse{
			{Id: "a", Namespaces: []string{"a", "b"}},
			{Id: "d", Namespaces: []string{"d"}},
		}, registrationResponse)
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

	resp, err := app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBody)

	var registrationResponse RegistrationResponse
	assert.NoError(t, json.Unmarshal(responseBody, &registrationResponse))
	assert.Equal(t, RegistrationResponse{Id: "a", Namespaces: []string{"a", "b"}}, registrationResponse)

	// modify index is not exposed in the response
	registrationService.EXPECT().GetByBaseline(gomock.Any(), gomock.Eq("a")).Return(
		&composite.CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(13))}, nil)

	resp, err = app.Test(req, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotContains(t, string(responseBody), "modifyIndex")

	assert.NoError(t, json.Unmarshal(responseBody, &registrationResponse))
	assert.Equal(t, RegistrationResponse{Id: "a", Namespaces: []string{"a", "b"}}, registrationResponse)

	// test to request non existing domain
	registrationService.EXPECT().GetByBaseline(gomock.Any(), "non-existing").Return(nil, nil)
	req2 := httptest.NewRequest("GET", "/test/non-existing", nil)

	resp, err = app.Test(req2, fiber.TestConfig{Timeout: time.Duration(100) * time.Millisecond})
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBody)
}
