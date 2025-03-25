package v1

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
// @ID GetOrCreateTopicV1
// @Tags V1
// @Produce  json
// @Param request body model.TopicRegistrationRespDto true "Request Body"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {object}    model.TopicRegistrationRespDto
// @Success 200 {object}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Failure 410 {object}	map[string]string
// @Router /api/v1/kafka/topic [post]
func (c *TopicController) GetOrCreateTopic(fiberCtx *fiber.Ctx, topicRegistrationReqDto *model.TopicRegistrationReqDto) error {
	return c.TopicController.GetOrCreateTopic(fiberCtx, topicRegistrationReqDto, func(dto *model.TopicRegistrationRespDto) {
		if dto != nil && dto.RequestedSettings != nil {
			dto.RequestedSettings.MinNumPartitions = nil
		}
	})
}

// @Summary Search Topics using Agent Role
// @Description Search Topics
// @ID SearchTopicsV1
// @Tags V1
// @Produce  json
// @Param request body model.TopicSearchRequest true "TopicSearchRequest"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {array}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Router /api/v1/kafka/topic/search [post]
func (c *TopicController) SearchTopics(fiberCtx *fiber.Ctx) error {
	return c.TopicController.SearchTopics(fiberCtx, func(dtos []*model.TopicRegistrationRespDto) {
		for _, dto := range dtos {
			if dto != nil && dto.RequestedSettings != nil {
				dto.RequestedSettings.MinNumPartitions = nil
			}
		}
	})
}

// @Summary Get Topic By Classifier using Agent Role
// @Description Get Topic By Classifier
// @ID GetTopicByClassifierV1
// @Tags V1
// @Produce  json
// @Param request body  map[string]interface{} true "Request body"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {object}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 403 {object}	map[string]string
// @Router /api/v1/kafka/topic/get-by-classifier [post]
func (c *TopicController) GetTopicByClassifier(fiberCtx *fiber.Ctx) error {
	return c.TopicController.GetTopicByClassifier(fiberCtx, func(dto *model.TopicRegistrationRespDto) {
		if dto != nil && dto.RequestedSettings != nil {
			dto.RequestedSettings.MinNumPartitions = nil
		}
	})
}
