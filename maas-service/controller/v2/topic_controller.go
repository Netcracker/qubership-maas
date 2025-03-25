package v2

import (
	"github.com/gofiber/fiber/v2"
	"maas/maas-service/controller"
	"maas/maas-service/model"
	"maas/maas-service/service/auth"
	"maas/maas-service/service/kafka"
)

type TopicController struct {
	controller.TopicController
}

func NewTopicController(service kafka.KafkaService, authService auth.AuthService) *TopicController {
	return &TopicController{*controller.NewTopicController(service, authService)}
}

// @Summary Get Or Create Topic using Agent Role
// @Description Get Or Create Topic
// @ID GetOrCreateTopicV2
// @Tags V2
// @Produce  json
// @Param request body model.TopicTemplate true "Request Body"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Param onTopicExists query string true "onTopicExists"
// @Security BasicAuth[agent]
// @Success 200 {object}    model.TopicRegistrationRespDto
// @Success 201 {object}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Failure 410 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/kafka/topic [post]
func (c *TopicController) GetOrCreateTopic(fiberCtx *fiber.Ctx, topicRegistrationReqDto *model.TopicRegistrationReqDto) error {
	return c.TopicController.GetOrCreateTopic(fiberCtx, topicRegistrationReqDto, nil)
}

// @Summary Search Topics using Agent Role
// @Description Search Topics
// @ID SearchTopicsV2
// @Tags V2
// @Produce  json
// @Param request body model.TopicSearchRequest true "TopicSearchRequest"
// @Security BasicAuth[agent]
// @Success 200 {array}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/kafka/topic/search [post]
func (c *TopicController) SearchTopics(fiberCtx *fiber.Ctx) error {
	return c.TopicController.SearchTopics(fiberCtx, nil)
}

// @Summary Get Topic By Classifier using Agent Role
// @Description Get Topic By Classifier
// @ID GetTopicByClassifierV2
// @Tags V2
// @Produce  json
// @Param request body model.Classifier true "Classifier"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {array}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Failure 410 {object}	map[string]string
// @Router /api/v2/kafka/topic/get-by-classifier [post]
func (c *TopicController) GetTopicByClassifier(fiberCtx *fiber.Ctx) error {
	return c.TopicController.GetTopicByClassifier(fiberCtx, nil)
}
