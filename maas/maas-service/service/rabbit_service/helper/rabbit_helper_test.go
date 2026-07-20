package helper

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-resty/resty/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	mock_helper "github.com/netcracker/qubership-maas/service/rabbit_service/helper/mock"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/assert"
)

var (
	assertion *assert.Assertions
	vhost     model.VHostRegistration
	instance  model.RabbitInstance
)

func testInit(rabbit *testharness.TestRabbit) {
	instance = model.RabbitInstance{
		Id:       "1",
		ApiUrl:   rabbit.ApiUrl(),
		AmqpUrl:  rabbit.AmqpUrl(),
		User:     "guest",
		Password: "guest",
		Default:  true,
	}

	vhost = model.VHostRegistration{
		Id:         1,
		Vhost:      "test-vhost",
		User:       "user",
		Password:   "user",
		Namespace:  "namespace",
		InstanceId: "1",
	}
}

func TestRabbitVhostHelperImpl_FormatCnnUrl(t *testing.T) {
	assert_ := assert.New(t)

	rh := NewRabbitHelper(model.RabbitInstance{
		AmqpUrl: "ampq://rabbitmq.rabbitmq-core-dev:15672",
	}, *new(model.VHostRegistration))
	vhost := "3a0f6ba2ed4a4e5a913a063a66f666bf"
	expected := "ampq://rabbitmq.rabbitmq-core-dev:15672/3a0f6ba2ed4a4e5a913a063a66f666bf"

	response := rh.FormatCnnUrl(vhost)
	assert_.Equal(expected, response)
}

func TestRabbitVhostHelperImpl_CreateQueue(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		*new(model.RabbitInstance),
		*new(model.VHostRegistration),
		httpHelper,
	)

	queue := map[string]interface{}{
		"name": "test-queue",
	}

	queueBytes, err := json.Marshal(queue)
	assert.NoError(err)
	buf := gbytes.NewBuffer()
	_, err = buf.Write(queueBytes)
	assert.NoError(err)

	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			Request: &resty.Request{URL: "url"},
			RawResponse: &http.Response{
				StatusCode: http.StatusOK,
			}}, nil).
		Times(1)
	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			RawResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       buf,
			},
		}, nil).
		Times(1)

	resp, _, err := rabbitHelper.CreateQueue(context.Background(), queue)
	assert.NoError(err)
	assert.Equal("test-queue", (*resp.(*map[string]interface{}))["name"])
}

func TestRabbitVhostHelperImpl_CreateQueueInequivArg(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		*new(model.RabbitInstance),
		*new(model.VHostRegistration),
		httpHelper,
	)

	queue := map[string]interface{}{
		"name": "test-queue",
	}

	queueBytes, err := json.Marshal(queue)
	assert.NoError(err)
	buf := gbytes.NewBuffer()
	_, err = buf.Write(queueBytes)
	assert.NoError(err)

	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &RabbitHttpError{
			Code:          http.StatusBadRequest,
			ExpectedCodes: []int{http.StatusOK},
			Message:       "error during creating queue",
			Response:      []byte("{\"reason\": \"inequivalent arg\"}"),
		}).
		Times(1)
	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "DELETE", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			RawResponse: &http.Response{
				StatusCode: http.StatusNoContent,
				Body:       buf,
			},
		}, nil).
		Times(1)
	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			Request: &resty.Request{URL: "url"},
			RawResponse: &http.Response{
				StatusCode: http.StatusOK,
			}}, nil).
		Times(1)
	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			RawResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       buf,
			},
		}, nil).
		Times(1)

	resp, _, err := rabbitHelper.CreateQueue(context.Background(), queue)
	assert.NoError(err)
	assert.Equal("test-queue", (*resp.(*map[string]interface{}))["name"])
}

func TestRabbitVhostHelperImpl_CreateQueueBadRequest(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		*new(model.RabbitInstance),
		*new(model.VHostRegistration),
		httpHelper,
	)

	queue := map[string]interface{}{
		"name": "test-queue",
	}

	queueBytes, err := json.Marshal(queue)
	assert.NoError(err)
	buf := gbytes.NewBuffer()
	_, err = buf.Write(queueBytes)
	assert.NoError(err)

	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, &RabbitHttpError{
			Code:          http.StatusBadRequest,
			ExpectedCodes: []int{200},
			Message:       "bad request",
			Response:      nil,
		}).
		Times(1)

	_, _, err = rabbitHelper.CreateQueue(context.Background(), queue)
	assert.Error(err)
}

func createVhost(rabbit *testharness.TestRabbit) {
	testInit(rabbit)

	rabbitHelper := NewRabbitHelper(
		instance,
		vhost,
	)

	_, err := rabbitHelper.CreateVHostAndReturnStatus(context.Background())
	assertion.Nil(err)
}

func TestRabbitHelperImpl_test(t *testing.T) {
	assertion = assert.New(t)

	testharness.WithNewTestRabbit(t, func(rabbit *testharness.TestRabbit) {
		createVhost(rabbit)

		rabbitHelper := NewRabbitHelper(
			instance,
			vhost,
		)

		policy := map[string]interface{}{
			"name":       "p",
			"pattern":    "^amq\\.",
			"definition": map[string]interface{}{"expires": 18000000},
		}

		exch := map[string]interface{}{
			"name": "e",
		}

		exch2 := map[string]interface{}{
			"name": "e2",
		}

		q := map[string]interface{}{
			"name": "q",
		}

		b := map[string]interface{}{
			"source":      "e",
			"destination": "q",
		}

		b2 := map[string]interface{}{
			"source":      "e",
			"destination": "e2",
		}

		err := IsInstanceAvailable(&instance)
		assertion.NoError(err)

		_, _, err = rabbitHelper.CreatePolicy(context.Background(), policy)
		assertion.NoError(err)
		_, _, err = rabbitHelper.CreateExchange(context.Background(), exch)
		assertion.NoError(err)
		_, _, err = rabbitHelper.CreateExchange(context.Background(), exch2)
		assertion.NoError(err)
		_, _, err = rabbitHelper.CreateQueue(context.Background(), q)
		assertion.NoError(err)
		_, _, err = rabbitHelper.CreateBinding(context.Background(), b)
		assertion.NoError(err)
		_, _, err = rabbitHelper.CreateExchangeBinding(context.Background(), b2)
		assertion.NoError(err)
		_, err = rabbitHelper.CreateNormalOrLazyBinding(context.Background(), b)
		assertion.NoError(err)

		_, err = rabbitHelper.GetAllEntities(context.Background())
		assertion.NoError(err)
		_, err = rabbitHelper.GetAllExchanges(context.Background())
		assertion.NoError(err)
		_, err = rabbitHelper.GetExchangesStartsWithString(context.Background(), "e")
		assertion.NoError(err)
		_, err = rabbitHelper.GetExchangeSourceBindings(context.Background(), exch)
		assertion.NoError(err)

		_, err = rabbitHelper.DeleteBinding(context.Background(), b)
		assertion.NoError(err)
		_, err = rabbitHelper.DeleteExchangeBinding(context.Background(), b2)
		assertion.NoError(err)
		_, err = rabbitHelper.DeleteQueue(context.Background(), q)
		assertion.NoError(err)
		_, err = rabbitHelper.DeleteExchange(context.Background(), exch)
		assertion.NoError(err)
		_, err = rabbitHelper.DeleteExchange(context.Background(), exch2)
		assertion.NoError(err)
		_, err = rabbitHelper.DeletePolicy(context.Background(), policy)
		assertion.NoError(err)

		err = rabbitHelper.DeleteVHost(context.Background())
		assertion.NoError(err)
	})

}

func TestIsInstanceAvailable_healthCheckPaths(t *testing.T) {
	okBody := `{"status":"ok"}`

	t.Run("modern endpoint", func(t *testing.T) {
		var legacyCalled bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, rabbitHealthCheckAlarmsPath):
				_, _ = w.Write([]byte(okBody))
			case strings.HasSuffix(r.URL.Path, rabbitHealthCheckLegacyPath):
				legacyCalled = true
				w.WriteHeader(http.StatusNotFound)
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)

		helper := rabbitHelperForHealthTest(t, srv.URL)
		assert.NoError(t, helper.IsInstanceAvailable())
		assert.False(t, legacyCalled)
	})

	t.Run("fallback to legacy on 404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch {
			case strings.HasSuffix(r.URL.Path, rabbitHealthCheckAlarmsPath):
				w.WriteHeader(http.StatusNotFound)
			case strings.HasSuffix(r.URL.Path, rabbitHealthCheckLegacyPath):
				_, _ = w.Write([]byte(okBody))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		t.Cleanup(srv.Close)

		helper := rabbitHelperForHealthTest(t, srv.URL)
		assert.NoError(t, helper.IsInstanceAvailable())
	})

	t.Run("no fallback on unauthorized", func(t *testing.T) {
		var legacyCalled bool
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, rabbitHealthCheckLegacyPath) {
				legacyCalled = true
			}
			w.WriteHeader(http.StatusUnauthorized)
		}))
		t.Cleanup(srv.Close)

		helper := rabbitHelperForHealthTest(t, srv.URL)
		err := helper.IsInstanceAvailable()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "unauthorized")
		assert.False(t, legacyCalled)
	})
}

func rabbitHelperForHealthTest(t *testing.T, apiURL string) RabbitHelperImpl {
	t.Helper()
	return RabbitHelperImpl{
		instance: model.RabbitInstance{
			ApiUrl:   apiURL + "/api",
			User:     "guest",
			Password: "guest",
		},
		httpClient: utils.NewRestyClient(),
	}
}

func TestRabbitVhostHelperImpl_GetAllVhosts(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		*new(model.RabbitInstance),
		*new(model.VHostRegistration),
		httpHelper,
	)

	body := gbytes.NewBuffer()
	_, err := body.Write([]byte(`[{"name":"maas.core-dev.orders"},{"name":"/"}]`))
	assert.NoError(err)

	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			RawResponse: &http.Response{
				StatusCode: http.StatusOK,
				Body:       body,
			},
		}, nil).
		Times(1)

	vhosts, err := rabbitHelper.GetAllVhosts(context.Background())
	assert.NoError(err)
	assert.Len(vhosts, 2)
	assert.Equal("maas.core-dev.orders", vhosts[0].Name)
	assert.Equal("/", vhosts[1].Name)
}

func TestRabbitVhostHelperImpl_GetAllVhostsNotFound(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		*new(model.RabbitInstance),
		*new(model.VHostRegistration),
		httpHelper,
	)

	httpHelper.EXPECT().
		DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(&resty.Response{
			RawResponse: &http.Response{StatusCode: http.StatusNotFound},
		}, nil).
		Times(1)

	vhosts, err := rabbitHelper.GetAllVhosts(context.Background())
	assert.NoError(err)
	assert.Nil(vhosts)
}
