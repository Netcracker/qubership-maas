package controller

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/monitoring"
	mock_instance "github.com/netcracker/qubership-maas/service/instance/mock"
	"github.com/netcracker/qubership-maas/service/kafka"
	mock_helper "github.com/netcracker/qubership-maas/service/kafka/helper/mock"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscrepancyController(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	mockCtrl := gomock.NewController(t)
	dao := kafka.NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_helper.NewMockHelper(mockCtrl)
	auditService := monitoring.NewMockAuditor(mockCtrl)
	eventBus := kafka.NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	kafkaService := kafka.NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, nil)

	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Namespace: "test-ns",
		})).
		Return([]*model.TopicRegistration{
			{
				Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
				Topic:      "first",
			},
			{
				Classifier: &model.Classifier{Name: "second-test-topic", Namespace: "test-ns", TenantId: "42"},
				Topic:      "second",
			},
			{
				Classifier: &model.Classifier{Name: "third-test-topic", Namespace: "test-ns", TenantId: "42"},
				Topic:      "third",
			},
		}, nil).
		AnyTimes()
	instanceService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).Return(&model.KafkaInstance{
		Id: "42",
	}, nil).
		AnyTimes()

	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("first")).
		Return(true, nil).
		AnyTimes()
	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("second")).
		Return(true, nil).
		AnyTimes()
	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("third")).
		Return(false, nil).
		AnyTimes()

	app.Get("/discrepancy/report/:namespace", NewDiscrepancyController(kafkaService).GetReport)

	req := httptest.NewRequest("GET", "/discrepancy/report/test-ns", nil)
	resp, _ := app.Test(req)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	report := getReportFromBody(t, resp.Body)

	assert.Equal(t, 3, len(report))

	assert.Equal(t, "first", report[0]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "first-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[0]["classifier"])
	assert.Equal(t, model.StatusOk, report[0]["status"])

	assert.Equal(t, "second", report[1]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "second-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[1]["classifier"])
	assert.Equal(t, model.StatusOk, report[1]["status"])

	assert.Equal(t, "third", report[2]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "third-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[2]["classifier"])
	assert.Equal(t, model.StatusAbsent, report[2]["status"])

	req = httptest.NewRequest("GET", "/discrepancy/report/test-ns?status=ok", nil)
	resp, _ = app.Test(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	report = getReportFromBody(t, resp.Body)

	assert.Equal(t, 2, len(report))

	assert.Equal(t, "first", report[0]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "first-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[0]["classifier"])
	assert.Equal(t, model.StatusOk, report[0]["status"])

	assert.Equal(t, "second", report[1]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "second-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[1]["classifier"])
	assert.Equal(t, model.StatusOk, report[1]["status"])

	req = httptest.NewRequest("GET", "/discrepancy/report/test-ns?status=absent", nil)
	resp, _ = app.Test(req)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	report = getReportFromBody(t, resp.Body)

	assert.Equal(t, 1, len(report))

	assert.Equal(t, "third", report[0]["name"])
	assert.EqualValues(t, map[string]interface{}{"name": "third-test-topic", "namespace": "test-ns", "tenantId": "42"}, report[0]["classifier"])
	assert.Equal(t, model.StatusAbsent, report[0]["status"])
}

func getReportFromBody(t assert.TestingT, bodyRead io.ReadCloser) []map[string]interface{} {
	body, err := io.ReadAll(bodyRead)
	assert.Nil(t, err)
	err = bodyRead.Close()
	assert.Nil(t, err)
	var report []map[string]interface{}
	err = json.Unmarshal(body, &report)
	assert.Nil(t, err)

	return report
}
