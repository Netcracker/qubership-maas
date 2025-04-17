package controller

import (
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/bg_service"
	"github.com/netcracker/qubership-maas/utils"
	"net/http"
)

var bLog logging.Logger

func init() {
	bLog = logging.GetLogger("bg-controller")
}

type BgController struct {
	bgService   bg_service.BgService
	authService auth.AuthService
}

func NewBgController(s bg_service.BgService, a auth.AuthService) *BgController {
	return &BgController{s, a}
}

// @Summary Apply Bg Status using Agent Role
// @Description Apply Bg Status
// @ID ApplyBgStatus
// @Tags V1
// @Produce  json
// @Param request body model.CpMessageDto true "CpMessageDto"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v1/bg-status [post]
func (c *BgController) ApplyBgStatus(fiberCtx *fiber.Ctx) error {

	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace

	var cpMessageDto model.CpMessageDto
	if err := json.Unmarshal(fiberCtx.Body(), &cpMessageDto); err != nil {
		return utils.LogError(bLog, ctx, "error while unmarshalling cpMessage: %s: %w", err.Error(), msg.BadRequest)
	}

	cpMessage, err := cpMessageDto.ConvertToBgStatus()
	if err != nil {
		return utils.LogError(bLog, ctx, "error while ConvertToBgStatus for cpMessageDto '%v': %s: %w", cpMessageDto, err.Error(), msg.BadRequest)
	}

	bLog.InfoC(ctx, "Applying blue-green status: %+v", cpMessage)
	err = c.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	if err != nil {
		return utils.LogError(bLog, ctx, "error while ApplyBgStatus for cp message: %v: %s: %w", cpMessage, err.Error(), msg.BadRequest)
	}

	bLog.InfoC(ctx, "Successfully applied blue-green status")
	return RespondWithJson(fiberCtx, 200, nil)

}

// @Summary Get Bg Status By Namespace using Agent Role
// @Description Get Bg Status By Namespace
// @ID GetBgStatusByNamespace
// @Tags V1
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {object}    model.BgStatus
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v1/bg-status [get]
func (c *BgController) GetBgStatusByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	bLog.InfoC(ctx, "Received request to get bg status by namespace ctx: %+v", model.RequestContextOf(ctx))
	namespace := model.RequestContextOf(ctx).Namespace
	bgStatus, err := c.bgService.GetBgStatusByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(bLog, ctx, "can not get bg status by namespace '%s': %s", namespace, err.Error())
	}
	if bgStatus == nil {
		return utils.LogError(bLog, ctx, "no bg status found: %w", msg.NotFound)
	}
	return RespondWithJson(fiberCtx, http.StatusOK, bgStatus)
}
