package v1

import (
	"encoding/json"
	"github.com/netcracker/qubership-maas/controller"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"github.com/netcracker/qubership-maas/service/cr"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestCustomResourceController_Apply_Topic(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	customResourceProcessorService := NewMockCustomResourceProcessorService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewCustomResourceController(customResourceProcessorService)

	app.Post("/test", controller.WithYaml[cr.CustomResourceRequest](c.Create))

	customResourceProcessorService.EXPECT().
		Apply(gomock.Any(), &cr.CustomResourceRequest{
			ApiVersion: configurator_service.CoreNcV1ApiVersion,
			Kind:       "MaaS",
			SubKind:    configurator_service.CustomResourceKindTopic,
			Metadata: &cr.CustomResourceMetadataRequest{
				Name:      "test-topic",
				Namespace: "test-namespace",
			},
			Spec: &cr.CustomResourceSpecRequest{
				"pragma":        cr.CustomResourceSpecRequest(map[string]any{"onEntityExists": "merge"}),
				"numPartitions": 3,
				"instance":      "test-instance",
			},
		}, cr.ActionCreate)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: Topic
metadata:
  name: test-topic
  namespace: test-namespace
spec:
  pragma:
    onEntityExists: merge
  numPartitions: 3
  instance: test-instance
`))

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(responseBody), "{\"status\":\"COMPLETED\",\"conditions\":[{\"type\":\"Validated\",\"state\":\"COMPLETED\"},{\"type\":\"DependenciesResolved\",\"state\":\"COMPLETED\"},{\"type\":\"Created\",\"state\":\"COMPLETED\"}]}")

	reqNamespaceToLong := httptest.NewRequest("POST", "/test", strings.NewReader(`
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: Topic
metadata:
  name: test-topic
  namespace: abcd012345678901234567890123456789012345678901234567890123456789
spec:
  pragma:
    onEntityExists: merge
  numPartitions: 3
  instance: test-instance
`))

	resp, err = app.Test(reqNamespaceToLong, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err = io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(responseBody), "metadata.namespace length must be less than or equal to '63'")
}

func TestCustomResourceController_Apply_Topic_With_Template(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	customResourceProcessorService := NewMockCustomResourceProcessorService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewCustomResourceController(customResourceProcessorService)

	app.Post("/test", controller.WithYaml[cr.CustomResourceRequest](c.Create))

	customResourceProcessorService.EXPECT().
		Apply(gomock.Any(), &cr.CustomResourceRequest{
			ApiVersion: configurator_service.CoreNcV1ApiVersion,
			Kind:       "MaaS",
			SubKind:    configurator_service.CustomResourceKindTopic,
			Metadata: &cr.CustomResourceMetadataRequest{
				Name:      "test-topic",
				Namespace: "test-namespace",
			},
			Spec: &cr.CustomResourceSpecRequest{
				"pragma":        cr.CustomResourceSpecRequest(map[string]any{"onEntityExists": "merge"}),
				"numPartitions": 3,
				"instance":      "test-instance",
				"template":      "test-template",
			},
		}, cr.ActionCreate).Return(&cr.CustomResourceWaitEntity{
		TrackingId:     42,
		CustomResource: "test-resource",
		Namespace:      "test-namespace",
		Reason:         "test-reason",
		Status:         cr.CustomResourceStatusInProgress,
		CreatedAt:      time.Now(),
	}, nil)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`
apiVersion: core.netcracker.com/v1
kind: MaaS
subKind: Topic
metadata:
  name: test-topic
  namespace: test-namespace
spec:
  pragma:
    onEntityExists: merge
  numPartitions: 3
  instance: test-instance
  template: test-template
`))

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)

	responseBodyJson, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBodyJson)

	var responseBody cr.CustomResourceResponse
	err = json.Unmarshal(responseBodyJson, &responseBody)
	assert.NoError(t, err)

	assert.Equal(t, "IN_PROGRESS", string(responseBody.Status))
	assert.Equal(t, "DependenciesResolved", string(responseBody.Conditions[1].Type))
	assert.Equal(t, "test-reason", responseBody.Conditions[1].Message)

	assert.Equal(t, "NOT_STARTED", string(responseBody.Conditions[2].State))
	assert.Equal(t, "Created", string(responseBody.Conditions[2].Type))
	assert.Equal(t, "waiting for dependency", responseBody.Conditions[2].Reason)

	assert.Equal(t, int64(42), responseBody.TrackingId)
}

func TestCustomResourceController_Status(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	customResourceProcessorService := NewMockCustomResourceProcessorService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewCustomResourceController(customResourceProcessorService)

	app.Get("/test/:trackingId", c.Status)

	customResourceProcessorService.EXPECT().GetStatus(gomock.Any(), int64(42))

	req := httptest.NewRequest("GET", "/test/42", nil)

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	customResourceProcessorService.EXPECT().GetStatus(gomock.Any(), int64(42)).Return(&cr.CustomResourceWaitEntity{
		TrackingId:     42,
		CustomResource: "test-custom-resource",
		Namespace:      "test-namespace",
		Reason:         "test-reason",
		Status:         cr.CustomResourceStatusInProgress,
		CreatedAt:      time.Now(),
	}, nil)

	req = httptest.NewRequest("GET", "/test/42", nil)

	resp, err = app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBodyJson, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBodyJson)
	var responseBody cr.CustomResourceResponse
	err = json.Unmarshal(responseBodyJson, &responseBody)
	assert.NoError(t, err)

	assert.Equal(t, "IN_PROGRESS", string(responseBody.Status))
	assert.Equal(t, "DependenciesResolved", string(responseBody.Conditions[1].Type))
	assert.Equal(t, "test-reason", responseBody.Conditions[1].Message)

	assert.Equal(t, "NOT_STARTED", string(responseBody.Conditions[2].State))
	assert.Equal(t, "Created", string(responseBody.Conditions[2].Type))
	assert.Equal(t, "waiting for dependency", responseBody.Conditions[2].Reason)

	assert.Equal(t, int64(42), responseBody.TrackingId)
}

func TestCustomResourceController_Terminate(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	customResourceProcessorService := NewMockCustomResourceProcessorService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	c := NewCustomResourceController(customResourceProcessorService)

	app.Get("/test/:trackingId/terminate", c.Terminate)

	customResourceProcessorService.EXPECT().Terminate(gomock.Any(), int64(42))

	req := httptest.NewRequest("GET", "/test/42/terminate", nil)

	resp, err := app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	customResourceProcessorService.EXPECT().Terminate(gomock.Any(), int64(42)).Return(&cr.CustomResourceWaitEntity{
		TrackingId:     42,
		CustomResource: "test-custom-resource",
		Namespace:      "test-namespace",
		Reason:         "test-reason",
		Status:         cr.CustomResourceStatusTerminated,
		CreatedAt:      time.Now(),
	}, nil)

	req = httptest.NewRequest("GET", "/test/42/terminate", nil)

	resp, err = app.Test(req, 100)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBodyJson, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.NotEmpty(t, responseBodyJson)
	var responseBody cr.CustomResourceResponse
	err = json.Unmarshal(responseBodyJson, &responseBody)
	assert.NoError(t, err)

	assert.Equal(t, "TERMINATED", string(responseBody.Status))
	assert.Equal(t, "DependenciesResolved", string(responseBody.Conditions[1].Type))
	assert.Equal(t, "test-reason", responseBody.Conditions[1].Message)

	assert.Equal(t, "NOT_STARTED", string(responseBody.Conditions[2].State))
	assert.Equal(t, "Created", string(responseBody.Conditions[2].Type))
	assert.Equal(t, "waiting for dependency", responseBody.Conditions[2].Reason)

	assert.Equal(t, int64(42), responseBody.TrackingId)
}
