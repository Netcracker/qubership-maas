package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/service/auth"
	kafka2 "maas/maas-service/service/kafka"
	"maas/maas-service/service/kafka/helper"
	"maas/maas-service/utils"
	"maas/maas-service/validator"
	"net/http"
	"time"
)

var tLog = logging.GetLogger("topic-controller")

type TopicController struct {
	kafkaService kafka2.KafkaService
	authService  auth.AuthService
}

func NewTopicController(service kafka2.KafkaService, authService auth.AuthService) *TopicController {
	return &TopicController{kafkaService: service, authService: authService}
}

func (c *TopicController) GetOrCreateTopic(fiberCtx *fiber.Ctx, topicRegistrationReqDto *model.TopicRegistrationReqDto, beforeResponse func(*model.TopicRegistrationRespDto)) error {
	ctx := fiberCtx.UserContext()

	onTopicExists := model.Fail
	if fiberCtx.Query("onTopicExists") == "merge" {
		onTopicExists = model.Merge
	}

	topicTemplate, err := c.kafkaService.GetTopicTemplateByNameAndNamespace(ctx, topicRegistrationReqDto.Template, topicRegistrationReqDto.Classifier.Namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error resolve topic template: %s: %w", err.Error(), msg.BadRequest)
	}

	topic, err := topicRegistrationReqDto.BuildTopicRegFromReq(topicTemplate)
	if err != nil {
		return utils.LogError(log, ctx, "error resolve topic template settings: %s: %w", err.Error(), msg.BadRequest)
	}

	found, registeredTopic, err := c.kafkaService.GetOrCreateTopic(ctx, topic, onTopicExists)
	if err != nil {
		return utils.LogError(log, ctx, "error get or create topic: %w", err)
	}

	registeredTopic.ActualSettings.MinNumPartitions = nil
	if beforeResponse != nil {
		beforeResponse(registeredTopic)
	}

	if found {
		tLog.InfoC(ctx, "Found existing kafka topic '%s' in instance '%s'", registeredTopic.Name, registeredTopic.Instance)
		return RespondWithJson(fiberCtx, http.StatusOK, registeredTopic)
	} else {
		tLog.InfoC(ctx, "Created new kafka topic '%s' in instance '%s'", registeredTopic.Name, registeredTopic.Instance)
		return RespondWithJson(fiberCtx, http.StatusCreated, registeredTopic)
	}
}

func (c *TopicController) GetOrCreateLazyTopic(fiberCtx *fiber.Ctx) error {
	return ExtractAndValidateClassifier(fiberCtx, func(ctx context.Context, classifier *model.Classifier) error {
		onTopicExists := model.Fail
		if fiberCtx.Query("onTopicExists") == "merge" {
			onTopicExists = model.Merge
		}

		found, registeredTopic, err := c.kafkaService.GetOrCreateLazyTopic(ctx, classifier, onTopicExists)
		if err != nil {
			return utils.LogError(log, ctx, "error get or create lazy topic: %w", err)
		}

		if found {
			tLog.InfoC(ctx, "Found existing kafka topic '%s' in instance '%s'", registeredTopic.Name, registeredTopic.Instance)
			return RespondWithJson(fiberCtx, http.StatusOK, registeredTopic)
		} else {
			tLog.InfoC(ctx, "Created new kafka topic '%s' in instance '%s'", registeredTopic.Name, registeredTopic.Instance)
			return RespondWithJson(fiberCtx, http.StatusCreated, registeredTopic)
		}
	})
}

func (c *TopicController) GetTopicByClassifier(fiberCtx *fiber.Ctx, beforeResponse func(*model.TopicRegistrationRespDto)) error {
	return ExtractAndValidateClassifier(fiberCtx, func(ctx context.Context, classifier *model.Classifier) error {
		topic, err := c.kafkaService.GetTopicByClassifierWithBgDomain(ctx, *classifier)
		if err != nil {
			return utils.LogError(log, ctx, "error while searching for topic by classifier %+v: %w", classifier, err)
		}
		if topic == nil {
			return RespondWithJson(fiberCtx, http.StatusNotFound, nil)
		}

		if beforeResponse != nil {
			beforeResponse(topic)
		}

		return RespondWithJson(fiberCtx, http.StatusOK, topic)
	})
}

func (c *TopicController) SearchTopics(fiberCtx *fiber.Ctx, beforeResponse func([]*model.TopicRegistrationRespDto)) error {
	ctx := fiberCtx.UserContext()

	tLog.InfoC(ctx, "Received request to search topics by criteria %s, ctx: %+v", string(fiberCtx.Body()), model.RequestContextOf(ctx))
	var searchCriteria model.TopicSearchRequest
	if err := json.Unmarshal(fiberCtx.Body(), &searchCriteria); err != nil {
		return utils.LogError(log, ctx, "failed to read request body: %s: %w", err.Error(), msg.BadRequest)
	}

	result, err := c.kafkaService.SearchTopics(ctx, &searchCriteria)
	if err != nil {
		return utils.LogError(log, ctx, "search for topics failed with error: %w", err)
	}
	if beforeResponse != nil {
		beforeResponse(result)
	}
	return RespondWithJson(fiberCtx, http.StatusOK, result)

}

// @Summary Delete Topic using Agent Role
// @Description Delete Topic
// @ID DeleteTopic
// @Tags V1
// @Produce  json
// @Param request body model.TopicSearchRequest true "TopicSearchRequest"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {object}    model.TopicDeletionResp
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 405 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v1/kafka/topic [delete]
func (c *TopicController) DeleteTopic(fiberCtx *fiber.Ctx) error {
	var searchCriteria model.TopicSearchRequest
	return extractRequestDto(fiberCtx, &searchCriteria, func(ctx context.Context) error {
		tLog.InfoC(ctx, "Received request to delete topics %s ctx: %+v", string(fiberCtx.Body()), model.RequestContextOf(ctx))
		response, err := c.kafkaService.DeleteTopics(ctx, &searchCriteria)
		if err != nil {
			if err == kafka2.ErrSearchTopicAttemptWithEmptyCriteria {
				return utils.LogError(log, ctx, "can not delete topics: %s: %w", err.Error(), msg.BadRequest)
			}
			if err == helper.ErrInvalidDeleteRequest || err == helper.ErrTopicDeletionDisabled {
				return RespondWithJson(fiberCtx, http.StatusMethodNotAllowed, response)
			} else {
				// we have not deleted anything
				return utils.LogError(log, ctx, "can not delete topics: %s", err.Error())
			}
		} else if len(response.FailedToDelete) > 0 {
			return RespondWithJson(fiberCtx, http.StatusInternalServerError, response)
		} else {
			return RespondWithJson(fiberCtx, http.StatusOK, response)
		}

	})
}

// @Summary Sync All Topics To Kafka using Agent Role
// @Description Sync All Topics To Kafka
// @ID SyncAllTopicsToKafka
// @Tags V2
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {array}    model.KafkaTopicSyncReport
// @Failure 500 {object}	map[string]string
// @Router /api/v2/kafka/recovery/{namespace} [post]
func (c *TopicController) SyncAllTopicsToKafka(fiberCtx *fiber.Ctx) error {
	report, err := c.kafkaService.SyncAllTopicsToKafka(fiberCtx.UserContext(), fiberCtx.Params("namespace"))
	if err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "can not sync topics to kafka: %s", err.Error())
	}

	return RespondWithJson(fiberCtx, http.StatusOK, report)
}

// @Summary Sync Topic To Kafka using Agent Role
// @Description Sync Topic To Kafka
// @ID SyncTopicToKafka
// @Tags V2
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {array}    model.KafkaTopicSyncReport
// @Failure 500 {object}	map[string]string
// @Failure 403 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/kafka/recover-topic [post]
func (c *TopicController) SyncTopicToKafka(fiberCtx *fiber.Ctx) error {
	return ExtractAndValidateClassifier(fiberCtx, func(ctx context.Context, classifier *model.Classifier) error {
		report, err := c.kafkaService.SyncTopicToKafka(ctx, *classifier)
		if err != nil {
			return utils.LogError(log, ctx, "error sync topic by classifier: %+v: %w", classifier, err)
		}

		return RespondWithJson(fiberCtx, http.StatusOK, report)
	})
}

func (c *TopicController) GetKafkaTopicTemplatesByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	tLog.InfoC(ctx, "Received request to get all topic templates %s ctx: %+v", string(fiberCtx.Body()), model.RequestContextOf(ctx))
	templates, err := c.kafkaService.GetKafkaTopicTemplatesByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	if err != nil {
		return err
	}

	return RespondWithJson(fiberCtx, http.StatusOK, templates)
}

func (c *TopicController) DeleteTemplate(fiberCtx *fiber.Ctx) error {
	var topicTemplateDto model.TopicTemplateReqDto
	return extractRequestDto(fiberCtx, &topicTemplateDto, func(ctx context.Context) error {
		tLog.InfoC(ctx, "Received request to delete topic template %s ctx: %+v", string(fiberCtx.Body()), model.RequestContextOf(ctx))
		namespace := model.RequestContextOf(ctx).Namespace
		name := topicTemplateDto.Name
		template, err := c.kafkaService.DeleteTopicTemplate(ctx, name, namespace)
		if err == kafka2.ErrTopicTemplateIsUsedByTopic {
			return utils.LogError(log, ctx, "can not delete topic template %s: %w", err.Error(), msg.BadRequest)
		} else if err != nil {
			return utils.LogError(log, ctx, "can not delete topic template: %s", err.Error())
		}
		if template == nil {
			return utils.LogError(log, ctx, "topic template with name '%v' and namespace '%v' was not found: %w", name, namespace, msg.NotFound)
		}
		return RespondWithJson(fiberCtx, http.StatusOK, template)
	})
}

func (c *TopicController) DefineLazyTopic(fiberCtx *fiber.Ctx) error {
	var topicDto model.TopicRegistrationReqDto
	return extractRequestDto(fiberCtx, &topicDto, func(ctx context.Context) error {
		namespace := model.RequestContextOf(ctx).Namespace

		if err := validator.Get().Struct(&topicDto); err != nil {
			return utils.LogError(log, ctx, "validation error %s: %w", err.Error(), msg.BadRequest)
		}

		err := topicDto.Classifier.CheckAllowedNamespaces(namespace)
		if err != nil {
			return utils.LogError(log, ctx, "error in VerifyAuth of classifier during CreateLazyTopic: %s: %w", err.Error(), msg.AuthError)
		}

		lazyTopic, err := topicDto.ToTopicDefinition(namespace, model.TopicDefinitionKindLazy)
		if err != nil {
			return utils.LogError(log, ctx, "error while handling a request for kafka lazy topic '%s': %s: %w", topicDto.Name, err.Error(), msg.BadRequest)
		}

		found, lazyTopic, err := c.kafkaService.GetOrCreateTopicDefinition(ctx, lazyTopic)
		if err != nil {
			return fmt.Errorf("error while creating kafka lazy topic %s: %w", topicDto.Name, err)
		}

		err = validator.ValidateClassifier(lazyTopic.Classifier)
		if err != nil {
			return utils.LogError(log, ctx, "incorrect classifier: %w", err)
		}
		if found {
			tLog.InfoC(ctx, "Found existing kafka lazy topic with classifier '%+v'", lazyTopic.Classifier)
			return RespondWithJson(fiberCtx, http.StatusOK, lazyTopic.MakeTopicDefinitionResponse())
		} else {
			tLog.InfoC(ctx, "Created new kafka lazy topic with classifier '%+v'", lazyTopic.Classifier)
			return RespondWithJson(fiberCtx, http.StatusCreated, lazyTopic.MakeTopicDefinitionResponse())
		}
	})
}

func (c *TopicController) GetLazyTopicsByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	tLog.InfoC(ctx, "Received request to get all lazy topics by namespace ctx: %+v", model.RequestContextOf(ctx))
	topics, err := c.kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, model.RequestContextOf(ctx).Namespace, model.TopicDefinitionKindLazy)
	if err != nil {
		return err
	}
	return RespondWithJson(fiberCtx, http.StatusOK, topics)
}

func (c *TopicController) GetTenantTopicsByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	tLog.InfoC(ctx, "Received request to get all tenant topics by namespace ctx: %+v", model.RequestContextOf(ctx))
	topics, err := c.kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, model.RequestContextOf(ctx).Namespace, model.TopicDefinitionKindTenant)
	if err != nil {
		return err
	}
	if topics == nil {
		return utils.LogError(log, ctx, "no tenant topics found: %w", msg.NotFound)
	}
	return RespondWithJson(fiberCtx, http.StatusOK, topics)
}

func (c *TopicController) DeleteLazyTopic(fiberCtx *fiber.Ctx) error {
	return checkNamespacePermissions(fiberCtx, func(ctx context.Context, namespace string, classifier model.Classifier) error {
		lazyTopic, err := c.kafkaService.DeleteTopicDefinition(ctx, &classifier)
		if err != nil {
			return utils.LogError(log, ctx, "can not delete topic definition: %s", err.Error())
		}
		if lazyTopic == nil {
			return utils.LogError(log, ctx, "lazy topic with classifier '%v' and namespace '%v' was not found: %w", classifier, namespace, msg.NotFound)
		}
		// TODO migrate to v2 with .MakeTopicDefinitionResponse(): classifier should be as an object, not as string
		return RespondWithJson(fiberCtx, http.StatusOK, lazyTopic)
	})
}

func (c *TopicController) DeleteTenantTopic(fiberCtx *fiber.Ctx) error {
	return checkNamespacePermissions(fiberCtx, func(ctx context.Context, namespace string, classifier model.Classifier) error {
		tLog.InfoC(ctx, "Received request to delete tenant topic definition by `%+v', ctx: %+v", classifier, model.RequestContextOf(ctx))
		tenantTopic, err := c.kafkaService.DeleteTopicDefinition(ctx, &classifier)
		if err != nil {
			return err
		}
		if tenantTopic == nil {
			return utils.LogError(log, ctx, "tenant topic definition by classifier '%v' and namespace '%v' was not found: %w", classifier, namespace, msg.NotFound)
		}
		err = validator.ValidateClassifier(&classifier)
		if err != nil {
			return utils.LogError(log, ctx, "incorrect classifier: %w", err)
		}
		return RespondWithJson(fiberCtx, http.StatusOK, tenantTopic.MakeTopicDefinitionResponse())
	})
}

// @Summary Watch Topics Create using Agent Role
// @Description Watch Topics Create
// @ID WatchTopicsCreate
// @Tags V2
// @Produce  json
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Param timeout query string false "timeout"
// @Security BasicAuth[agent]
// @Success 200 {object}    model.TopicRegistrationRespDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 405 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/kafka/topic/watch-create [post]
func (c *TopicController) WatchTopicsCreate(fiberCtx *fiber.Ctx) error {
	requestTimeoutStr := fiberCtx.Query("timeout", "120s")
	requestTimeout, err := time.ParseDuration(requestTimeoutStr)
	if err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "invalid timeout value: %v: %s: %w", requestTimeoutStr, err.Error(), msg.BadRequest)
	}

	return extractNamespace(fiberCtx, func(namespace string) error {
		var classifiers []model.Classifier
		return extractRequestDto(fiberCtx, &classifiers, func(ctx context.Context) error {
			for _, classifier := range classifiers {
				if err := validator.Get().Var(classifier, validator.ClassifierTag); err != nil {
					return fmt.Errorf("classifier validation error: %w", err)
				}
				if err := classifier.CheckAllowedNamespaces(namespace); err != nil {
					return fmt.Errorf("classifier namespace reference error: %w", err)
				}
			}

			fiberCtx.Context().Response.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
			// Don't cache response:
			fiberCtx.Context().Response.Header.Set(fiber.HeaderCacheControl, "no-cache, no-store, must-revalidate") // HTTP 1.1.
			fiberCtx.Context().Response.Header.Set(fiber.HeaderPragma, "no-cache")                                  // HTTP 1.0.
			fiberCtx.Context().Response.Header.Set(fiber.HeaderExpires, "0")

			found, err := c.kafkaService.WatchTopicsCreate(ctx, classifiers, requestTimeout)
			if err != nil {
				return fmt.Errorf("error perform watch for topics: %w", err)
			}
			return RespondWithJson(fiberCtx, http.StatusOK, found)
		})
	})
}

func checkNamespacePermissions(fiberCtx *fiber.Ctx, f func(context.Context, string, model.Classifier) error) error {
	return extractNamespace(fiberCtx, func(namespace string) error {
		ctx := fiberCtx.UserContext()
		body := string(fiberCtx.Body())
		classifier, err := model.NewClassifierFromReq(body)
		if err != nil {
			return utils.LogError(log, ctx, "failed to create classifier from request body: %s: %s: %w", body, err.Error(), msg.BadRequest)
		}
		err = validator.ValidateClassifier(&classifier)
		if err != nil {
			return utils.LogError(log, ctx, "incorrect classifier: %w", err)
		}

		err = classifier.CheckAllowedNamespaces(namespace)
		if err != nil {
			return utils.LogError(log, ctx, "error check access permission to delete tenant topic definition by classifier %+v: %s: %w", classifier, err.Error(), msg.AuthError)
		}

		return f(ctx, namespace, classifier)
	})
}

func extractRequestDto(fiberCtx *fiber.Ctx, object any, handler func(ctx context.Context) error) error {
	ctx := fiberCtx.UserContext()

	body := fiberCtx.Body()
	if len(body) == 0 {
		return utils.LogError(log, ctx, "expected non empty request body: %w", msg.BadRequest)
	}

	if err := json.Unmarshal(body, object); err != nil {
		return utils.LogError(log, ctx, "failed to read request body: '%s': %s: %w", body, err.Error(), msg.BadRequest)
	}
	return handler(ctx)
}

func extractNamespace(fiberCtx *fiber.Ctx, handler func(namespace string) error) error {
	rc := model.RequestContextOf(fiberCtx.UserContext())
	if rc != nil {
		return handler(rc.Namespace)
	}

	return fmt.Errorf("missed request context object")
}
