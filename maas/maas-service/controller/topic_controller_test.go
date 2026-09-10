package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/kafka"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolveTemplateFailsWith reports the status a caller sees when the template
// lookup fails with cause.
func resolveTemplateFailsWith(t *testing.T, cause error) int {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	kafkaService := kafka.NewMockKafkaService(mockCtrl)
	kafkaService.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), "tpl", "test-namespace").
		Return(nil, cause)

	topicController := NewTopicController(kafkaService, nil)
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	app.Post("/topic", func(fiberCtx fiber.Ctx) error {
		return topicController.GetOrCreateTopic(fiberCtx, &model.TopicRegistrationReqDto{
			Classifier: model.Classifier{Name: "test", Namespace: "test-namespace"},
			Template:   "tpl",
		}, func(*model.TopicRegistrationRespDto) {})
	})

	response, err := app.Test(httptest.NewRequest(http.MethodPost, "/topic", nil))
	require.NoError(t, err)
	return response.StatusCode
}

// 400 is permanent: no client retries it, so the request would be lost for the
// whole switchover.
func TestGetOrCreateTopic_UnavailableDatabaseIsNotBadRequest(t *testing.T) {
	status := resolveTemplateFailsWith(t,
		fmt.Errorf("no such function: ANY: %w", dao.MasterDatabaseUnavailable))

	assert.Equal(t, http.StatusServiceUnavailable, status)
}

// A template that does not exist still is the caller's fault.
func TestGetOrCreateTopic_RejectedTemplateStaysBadRequest(t *testing.T) {
	status := resolveTemplateFailsWith(t, fmt.Errorf("topic template not found"))

	assert.Equal(t, http.StatusBadRequest, status)
}
