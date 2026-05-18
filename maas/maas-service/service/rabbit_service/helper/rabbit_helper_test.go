package helper

import (
	"context"
	"encoding/json"
	"github.com/go-resty/resty/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	mock_helper "github.com/netcracker/qubership-maas/service/rabbit_service/helper/mock"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/onsi/gomega/gbytes"
	"github.com/stretchr/testify/assert"
	"net/http"
	"testing"
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

	getShovelsBuf := gbytes.NewBuffer()
	_, err = getShovelsBuf.Write([]byte("[]"))
	assert.NoError(err)

	gomock.InOrder(
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, &RabbitHttpError{
				Code:          http.StatusBadRequest,
				ExpectedCodes: []int{http.StatusOK},
				Message:       "error during creating queue",
				Response:      []byte("{\"reason\": \"inequivalent arg\"}"),
			}),
		// deleteAssociatedShovels calls GET parameters/shovel before deleting the queue
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{
					StatusCode: http.StatusOK,
					Body:       getShovelsBuf,
				},
			}, nil),
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "DELETE", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{
					StatusCode: http.StatusNoContent,
					Body:       buf,
				},
			}, nil),
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				Request: &resty.Request{URL: "url"},
				RawResponse: &http.Response{
					StatusCode: http.StatusOK,
				}}, nil),
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{
					StatusCode: http.StatusOK,
					Body:       buf,
				},
			}, nil),
	)

	resp, _, err := rabbitHelper.CreateQueue(context.Background(), queue)
	assert.NoError(err)
	assert.Equal("test-queue", (*resp.(*map[string]interface{}))["name"])
}

// TestRabbitVhostHelperImpl_CreateQueueInequivArg_ShovelDeletedFirst verifies that when a queue is
// inequivalent and must be deleted+recreated, any shovel sourcing from that queue is deleted
// BEFORE the queue itself is deleted, so there is no gap where the queue is absent but the shovel exists.
func TestRabbitVhostHelperImpl_CreateQueueInequivArg_ShovelDeletedFirst(t *testing.T) {
	assert := assert.New(t)
	mockCtrl := gomock.NewController(t)
	httpHelper := mock_helper.NewMockHttpHelper(mockCtrl)

	const (
		srcVhost      = "source-vhost"
		exportedVhost = "exported-vhost"
		queueName     = "my-queue"
		shovelName    = "my-queue-ns-exported"
	)

	rabbitHelper := NewRabbitHelperWithHttpHelper(
		model.RabbitInstance{},
		model.VHostRegistration{Vhost: srcVhost},
		httpHelper,
	)

	queue := map[string]interface{}{"name": queueName}
	queueBytes, err := json.Marshal(queue)
	assert.NoError(err)

	// shovel in exported vhost sourcing from the queue in source-vhost
	shovels := []model.Shovel{{
		Name:  shovelName,
		Vhost: exportedVhost,
		Value: model.ShovelValue{
			SrcProtocol:  "amqp091",
			SrcUri:       "amqp://user:pass@rabbitmq/" + srcVhost,
			SrcQueue:     queueName,
			DestProtocol: "amqp091",
			DestUri:      "amqp://user:pass@rabbitmq/" + exportedVhost,
			DestQueue:    queueName,
		},
	}}
	shovelsBytes, err := json.Marshal(shovels)
	assert.NoError(err)

	shovelsBuf := gbytes.NewBuffer()
	_, err = shovelsBuf.Write(shovelsBytes)
	assert.NoError(err)

	queueBuf := gbytes.NewBuffer()
	_, err = queueBuf.Write(queueBytes)
	assert.NoError(err)

	// InOrder guarantees: shovel DELETE happens before queue DELETE
	gomock.InOrder(
		// 1. initial PUT → inequiv 400
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, &RabbitHttpError{
				Code:          http.StatusBadRequest,
				ExpectedCodes: []int{http.StatusOK},
				Message:       "inequivalent arg",
				Response:      []byte(`{"reason": "inequivalent arg"}`),
			}),
		// 2. GET parameters/shovel → returns the shovel referencing our queue
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusOK, Body: shovelsBuf},
			}, nil),
		// 3. DELETE parameters/shovel/exported-vhost/shovel-name — BEFORE queue deletion
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "DELETE", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody},
			}, nil),
		// 4. DELETE queue
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "DELETE", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusNoContent, Body: http.NoBody},
			}, nil),
		// 5. PUT queue recreate
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "PUT", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				Request:     &resty.Request{URL: "url"},
				RawResponse: &http.Response{StatusCode: http.StatusOK},
			}, nil),
		// 6. GET queue (post-create verification)
		httpHelper.EXPECT().
			DoRequest(gomock.Any(), "GET", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&resty.Response{
				RawResponse: &http.Response{StatusCode: http.StatusOK, Body: queueBuf},
			}, nil),
	)

	resp, _, err := rabbitHelper.CreateQueue(context.Background(), queue)
	assert.NoError(err)
	assert.Equal(queueName, (*resp.(*map[string]interface{}))["name"])
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
