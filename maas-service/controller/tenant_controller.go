package controller

import (
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/tenant"
	"github.com/netcracker/qubership-maas/utils"
	"net/http"
)

type TenantController struct {
	tenantService *tenant.TenantServiceImpl
	authService   auth.AuthService
}

func NewTenantController(s *tenant.TenantServiceImpl, a auth.AuthService) *TenantController {
	return &TenantController{s, a}
}

// @Summary Sync Tenants using Agent Role
// @Description Sync Tenants
// @ID SyncTenants
// @Tags V1
// @Produce  json
// @Param request body []model.Tenant true "Request Body"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {object}    []model.SyncTenantsResp
// @Failure 500 {object}	map[string]string
// @Router /api/v1/synchronize-tenants [post]
func (tc *TenantController) SyncTenants(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.DebugC(ctx, "Receive request to SyncTenants...")
	syncData := string(fiberCtx.Body())
	resp, err := tc.tenantService.ApplyTenants(ctx, syncData)
	if err != nil {
		return utils.LogError(log, ctx, "error while applying tenants with request `%v': %w ", syncData, err)
	}
	return RespondWithJson(fiberCtx, http.StatusOK, resp)
}

// @Summary Get Tenants By Namespace using Agent Role
// @Description Get Tenants By Namespace
// @ID GetTenantsByNamespace
// @Tags V1
// @Produce  json
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Security BasicAuth[agent]
// @Success 200 {array}    model.Tenant
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v1/tenants [get]
func (tc *TenantController) GetTenantsByNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.InfoC(ctx, "Received request to get all tenants by namespace ctx: %+v", model.RequestContextOf(ctx))
	tenants, err := tc.tenantService.GetTenantsByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	if err != nil {
		return err
	}
	return RespondWithJson(fiberCtx, http.StatusOK, tenants)
}
