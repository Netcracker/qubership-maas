package controller

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/instance"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/validator"
	"net/http"
)

type InstanceController struct {
	kafka       instance.KafkaInstanceService
	rabbit      instance.RabbitInstanceService
	authService auth.AuthService
}

var iLog logging.Logger

func init() {
	iLog = logging.GetLogger("instance_controller")
}

func NewInstanceController(kafkaInstanceService instance.KafkaInstanceService, rabbitInstanceService instance.RabbitInstanceService, a auth.AuthService) *InstanceController {
	return &InstanceController{kafka: kafkaInstanceService, rabbit: rabbitInstanceService, authService: a}
}

// @Summary Register New Rabbit Instance using Manager Role
// @Description Register New Rabbit Instance
// @ID RegisterNewRabbitInstance
// @Tags V1
// @Param request body model.RabbitInstance true "RabbitInstance"
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {object}    model.RabbitInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/rabbit/instance [post]
func (c *InstanceController) RegisterNewRabbitInstance(fiberCtx *fiber.Ctx) error {
	var input model.RabbitInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.rabbit.Register(ctx, &input) })
}

// @Summary Register New Kafka Instance using Manager Role
// @Description Register New Kafka Instance
// @ID RegisterNewKafkaInstance
// @Tags V1
// @Produce  json
// @Param request body model.KafkaInstance true "KafkaInstance"
// @Security BasicAuth[manager]
// @Success 200 {object}     model.KafkaInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/kafka/instance [post]
func (c *InstanceController) RegisterNewKafkaInstance(fiberCtx *fiber.Ctx) error {
	var input model.KafkaInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.kafka.Register(ctx, &input) })
}

// @Summary Get All Rabbit Instances using Manager Role
// @Description Get All Rabbit Instances
// @ID GetAllRabbitInstances
// @Tags V1
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {array}     model.RabbitInstance
// @Failure 500 {object}	map[string]string
// @Router /api/v1/rabbit/instances [get]
func (c *InstanceController) GetAllRabbitInstances(fiberCtx *fiber.Ctx) error {
	result, err := c.rabbit.GetRabbitInstances(fiberCtx.UserContext())
	if err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "can not list all rabbit instances: %s", err.Error())
	}
	return RespondWithJson(fiberCtx, http.StatusOK, result)
}

// @Summary Get All Kafka Instances using Manager Role
// @Description Get All Kafka Instances
// @ID GetAllKafkaInstances
// @Tags V1
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {array}     model.KafkaInstance
// @Failure 500 {object}	map[string]string
// @Router /api/v1/kafka/instances [get]
func (c *InstanceController) GetAllKafkaInstances(fiberCtx *fiber.Ctx) error {
	result, err := c.kafka.GetKafkaInstances(fiberCtx.UserContext())
	if err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "can not list all kafka instances: %s", err.Error())
	}
	return RespondWithJson(fiberCtx, http.StatusOK, result)
}

// @Summary Update Rabbit Instance using Manager Role
// @Description Update Rabbit Instance
// @ID UpdateRabbitInstance
// @Tags V1
// @Param request body model.RabbitInstance true "RabbitInstance"
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {object}    model.RabbitInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/rabbit/instance [put]
func (c *InstanceController) UpdateRabbitInstance(fiberCtx *fiber.Ctx) error {
	var input model.RabbitInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.rabbit.Update(ctx, &input) })
}

// @Summary Update New Kafka Instance using Manager Role
// @Description Update New Kafka Instance
// @ID UpdateKafkaInstance
// @Tags V1
// @Produce  json
// @Param request body model.KafkaInstance true "KafkaInstance"
// @Security BasicAuth[manager]
// @Success 200 {object}     model.KafkaInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/kafka/instance [put]
func (c *InstanceController) UpdateKafkaInstance(fiberCtx *fiber.Ctx) error {
	var input model.KafkaInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.kafka.Update(ctx, &input) })
}

// @Summary Set Default Rabbit Instance using Manager Role
// @Description Set Default Rabbit Instance
// @ID SetDefaultRabbitInstance
// @Tags V1
// @Param request body model.RabbitInstance true "RabbitInstance"
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {object}    model.RabbitInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/rabbit/instance/default [put]
func (c *InstanceController) SetDefaultRabbitInstance(fiberCtx *fiber.Ctx) error {
	var input model.RabbitInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.rabbit.SetDefault(ctx, &input) })
}

// @Summary Set Default Kafka Instance using Manager Role
// @Description Set Default Kafka Instance
// @ID SetDefaultKafkaInstance
// @Tags V1
// @Produce  json
// @Param request body model.KafkaInstance true "KafkaInstance"
// @Security BasicAuth[manager]
// @Success 200 {object}    model.KafkaInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/kafka/instance/default [put]
func (c *InstanceController) SetDefaultKafkaInstance(fiberCtx *fiber.Ctx) error {
	var input model.KafkaInstance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.kafka.SetDefault(ctx, &input) })
}

type Instance struct {
	Id string `json:"id" example:"maas-rabbitmq-code-dev"`
}

// @Summary Unregister Rabbit Instance using Manager Role
// @Description Unregister Rabbit Instance
// @ID UnregisterRabbitInstance
// @Tags V1
// @Param request body Instance true "Instance"
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {object}    model.RabbitInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/rabbit/instance [delete]
func (c *InstanceController) UnregisterRabbitInstance(fiberCtx *fiber.Ctx) error {
	var input Instance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.rabbit.Unregister(ctx, input.Id) })
}

// @Summary Unregister Kafka Instance using Manager Role
// @Description Unregister Kafka Instance
// @ID UnregisterKafkaInstance
// @Tags V1
// @Produce  json
// @Param request body Instance true "Instance"
// @Security BasicAuth[manager]
// @Success 200 {object}    model.KafkaInstance
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/kafka/instance [delete]
func (c *InstanceController) UnregisterKafkaInstance(fiberCtx *fiber.Ctx) error {
	var input Instance
	return c.executeOperation(fiberCtx, &input, func(ctx context.Context) (any, error) { return c.kafka.Unregister(ctx, input.Id) })
}

func (c *InstanceController) executeOperation(fiberCtx *fiber.Ctx, input any, serviceFunction func(context.Context) (any, error)) error {
	if err := json.Unmarshal(fiberCtx.Body(), &input); err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "error parse json request: %v: %w", err.Error(), msg.BadRequest)
	}

	if err := validator.Get().Struct(input); err != nil {
		return utils.LogError(log, fiberCtx.UserContext(), "request validation error: %w", errors.Join(err, msg.BadRequest))
	}

	result, err := serviceFunction(fiberCtx.UserContext())
	if err != nil {
		return err
	}

	return RespondWithJson(fiberCtx, http.StatusOK, result)
}

func (c *InstanceController) GetKafkaInstanceDesignatorsByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace

	instanceDesignators, err := c.kafka.GetKafkaInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error while getting instance designators by namespace '%v': %w", namespace, err)
	}

	if instanceDesignators == nil {
		return utils.LogError(log, ctx, "no instance designators defined for namespace `%v': %w", namespace, msg.NotFound)
	}

	log.InfoC(ctx, "Getting kafka instance designator by namespace '%v' was successful", namespace)
	return RespondWithJson(fiberCtx, http.StatusOK, instanceDesignators)
}

func (c *InstanceController) DeleteKafkaInstanceDesignatorByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace

	err := c.kafka.DeleteKafkaInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error while deleting instance designator by namespace `%v': %w", namespace, err)
	}

	log.InfoC(ctx, "Deleting kafka instance designator by namespace '%v' was successful", namespace)
	return RespondWithJson(fiberCtx, http.StatusOK, nil)
}

func (c *InstanceController) GetRabbitInstanceDesignatorsByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace

	instanceDesignators, err := c.rabbit.GetRabbitInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error while getting instance designators by namespace '%v': %w", namespace, err)
	}

	if instanceDesignators == nil {
		return utils.LogError(log, ctx, "no instance designators defined for namespace `%v': %w", namespace, msg.NotFound)
	}

	log.InfoC(ctx, "Getting rabbit instance designator by namespace '%v' was successful", namespace)
	return RespondWithJson(fiberCtx, http.StatusOK, instanceDesignators)
}

func (c *InstanceController) DeleteRabbitInstanceDesignatorByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace

	err := c.rabbit.DeleteRabbitInstanceDesignatorByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error deleting rabbit instance designator by namespace '%v': %w", namespace, err)
	}

	log.InfoC(ctx, "Deleting rabbit instance designator by namespace '%v' was successful", namespace)
	return RespondWithJson(fiberCtx, http.StatusOK, nil)
}
