package controller

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"net/http"
)

type ConfiguratorController struct {
	configuratorService configurator_service.ConfiguratorService
	authService         auth.AuthService
}

func NewConfiguratorController(s configurator_service.ConfiguratorService, a auth.AuthService) *ConfiguratorController {
	return &ConfiguratorController{s, a}
}

// @Summary Apply Config using Agent Role
// @Description Apply Config
// @ID ApplyConfig
// @Tags V1
// @Produce  json
// @Param request body string true "Request Body"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security ApiKeyAuth
// @Success 200 {object}    interface{}
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v1/config [post]
func (c *ConfiguratorController) ApplyConfig(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Receive request to apply configs...")
	namespace := model.RequestContextOf(ctx).Namespace
	result, err := c.configuratorService.ApplyConfig(ctx, string(fiberCtx.Body()), namespace)
	var status int

	if result != nil && err == nil {
		status = http.StatusOK
	} else if result != nil && err != nil {
		log.ErrorC(ctx, fmt.Sprintf("Result != nil with error: %v", err.Error()))
		status = http.StatusInternalServerError
	} else if result == nil && err != nil {
		// may be request body is unparseable
		log.ErrorC(ctx, fmt.Sprintf("Result == nil with error: %v", err.Error()))
		status = http.StatusBadRequest
	} else {
		panic("Logic error")
	}

	var body interface{}
	if result != nil {
		body = result
	} else {
		body = err.Error()
	}
	return RespondWithJson(fiberCtx, status, body)
}

// @Summary Apply Config V2 using Agent Role
// @Description Apply Config V2
// @ID ApplyConfigV2
// @Tags V2
// @Produce  json
// @Param request body string true "Request Body"
// @Security BasicAuth[agent]
// @Success 200 {object}    interface{}
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/config [post]
func (c *ConfiguratorController) ApplyConfigV2(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Receive request to apply configs...")
	msResponses, err := c.configuratorService.ApplyConfigV2(ctx, string(fiberCtx.Body()))

	result := model.ConfigApplicationResponse{MsResponses: msResponses}
	if err != nil {
		log.ErrorC(ctx, fmt.Sprintf("Error in config controller during ApplyConfigV2: %v", err.Error()))
		result.Status = model.STATUS_ERROR
		result.Error = err.Error()

		configErr, ok := err.(model.AggregateConfigError)
		if !ok {
			return RespondWithJson(fiberCtx, http.StatusInternalServerError, result)
		}

		switch configErr.Err {
		case model.ErrAggregateConfigOverallRabbitError:
			return RespondWithJson(fiberCtx, http.StatusBadRequest, result)
		case model.ErrAggregateConfigParsing:
			return RespondWithJson(fiberCtx, http.StatusBadRequest, result)
		case model.ErrAggregateConfigInternalMsError:
			return RespondWithJson(fiberCtx, http.StatusInternalServerError, result)
		case model.ErrAggregateConfigValidationRabbitError:
			return RespondWithJson(fiberCtx, http.StatusConflict, result)
		default:
			return RespondWithJson(fiberCtx, http.StatusInternalServerError, result)
		}
	}

	result.Status = model.STATUS_OK
	return RespondWithJson(fiberCtx, http.StatusOK, result)
}
