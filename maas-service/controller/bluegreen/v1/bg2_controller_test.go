package v1

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"io"
	"maas/maas-service/controller"
	mock_bg2 "maas/maas-service/controller/bluegreen/v1/mock"
	"maas/maas-service/service/bg2/domain"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

var (
	app *fiber.App
)

func TestMain(m *testing.M) {
	app = fiber.New(fiber.Config{ErrorHandler: controller.TmfErrorHandler})
	os.Exit(m.Run())
}

func AssertResponseEqual(t *testing.T, expected any, body io.ReadCloser) {
	raw, err := io.ReadAll(body)
	assert.NoError(t, err, "error read response body")

	exp, err := json.Marshal(expected)
	assert.NoError(t, err, "can't serialize expected value")

	assert.Equal(t, string(exp), string(raw))
}

func TestBg2Controller_InitDomain(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	manager := mock_bg2.NewMockManager(mockCtrl)
	manager.EXPECT().
		InitDomain(gomock.Any(), gomock.Any()).
		Times(1)

	app.Post("/operation/init-domain", controller.WithJson(NewController(manager).InitDomain))

	payload := `{
  "BGState": {
    "originNamespace": {
        "name": "first-namespace",
        "state": "active",
        "version": "v5"
      },
      "peerNamespace": {
        "name": "second-namespace",
        "state": "candidate",
        "version": "v6"
      },
      "updateTime": "2023-07-07T12:00:54Z"
  }
}`
	req := httptest.NewRequest("POST", "/operation/init-domain", strings.NewReader(payload))
	resp, _ := app.Test(req, 10000000)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertResponseEqual(t, SyncResponse{Status: ProcessStatusCompleted, Message: "Init successfully finished"}, resp.Body)

	payload = `
{
  "BGState": {
    "originNamespace": {
        "name": "first-namespace",
        "state": "active",
        "version": "v5wrong"
      },
      "updateTime": "2023-07-07T12:00:54Z"
  }
}
`
	req = httptest.NewRequest("POST", "/operation/init-domain", strings.NewReader(payload))
	resp, _ = app.Test(req, 10000000)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	//FIXME
	assert.Contains(t, string(responseBody), "bgstate.peer is required")
	assert.Contains(t, string(responseBody), "must have /v\\\\d+ pattern")
}

func TestBg2Controller_Warmup(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	manager := mock_bg2.NewMockManager(mockCtrl)

	manager.EXPECT().
		Warmup(gomock.Any(), gomock.Any()).
		Times(1)

	app.Post("/operation/warmup", controller.WithJson(NewController(manager).Warmup))

	payload := `
{
  "BGState": {
    "originNamespace": {
        "name": "first-namespace",
        "state": "active",
        "version": "v5"
      },
      "peerNamespace": {
        "name": "second-namespace",
        "state": "candidate",
        "version": "v6"
      },
      "updateTime": "2023-07-07T12:00:54Z"
  }
}
`
	req := httptest.NewRequest("POST", "/operation/warmup", strings.NewReader(payload))
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertResponseEqual(t, SyncResponse{Status: ProcessStatusCompleted, Message: "Warmup successfully finished"}, resp.Body)

	payload = `
{
  "BGState": {
    "originNamespace": {
        "name": "first-namespace",
        "state": "active",
        "version": "v5wrong"
      },
      "updateTime": "2023-07-07T12:00:54Z"
  }
}
`
	req = httptest.NewRequest("POST", "/operation/warmup", strings.NewReader(payload))
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	responseBody, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	assert.Contains(t, string(responseBody), "bgstate.peer is required")
	assert.Contains(t, string(responseBody), "must have /v\\\\d+ pattern")
}

func TestBg2Controller_Commit(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	manager := mock_bg2.NewMockManager(mockCtrl)
	app.Post("/operation/commit", controller.WithJson(NewController(manager).Commit))

	manager.EXPECT().
		Commit(gomock.Any(), gomock.Any()).
		Times(1)

	payload := `
{
  "BGState": {
    "originNamespace": {
        "name": "first-namespace",
        "state": "active",
        "version": "v5"
      },
      "peerNamespace": {
        "name": "second-namespace",
        "state": "idle",
        "version": null
      },
      "updateTime": "2023-07-07T12:00:54Z"
  }
}
`

	req := httptest.NewRequest("POST", "/operation/commit", strings.NewReader(payload))
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	AssertResponseEqual(t, SyncResponse{Status: ProcessStatusCompleted, Message: "Commit successfully finished"}, resp.Body)
}

func TestBg2Controller_ListDomains(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	manager := mock_bg2.NewMockManager(mockCtrl)
	app.Get("/operation/list-domains", NewController(manager).ListDomains)

	manager.EXPECT().
		ListDomains(gomock.Any()).
		Return([]domain.BGNamespaces{
			{
				Origin:              "origin-1",
				Peer:                "peer-1",
				ControllerNamespace: "controller-1",
			},
			{
				Origin:              "origin-2",
				Peer:                "peer-2",
				ControllerNamespace: "controller-2",
			},
		}, nil).
		Times(1)

	req := httptest.NewRequest("GET", "/operation/list-domains", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	responseBody, err := io.ReadAll(resp.Body)
	var result []domain.BGNamespaces
	assert.NoError(t, json.Unmarshal(responseBody, &result))
	assert.Equal(t, 2, len(result))
}
