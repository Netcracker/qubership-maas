package v1

import (
	"context"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/controller"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/rabbit_service"
	"maas/maas-service/utils"
	"net/http"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("bg2-controller")
}

type Controller struct {
	bgManager Manager
}

//go:generate mockgen -source=bg2_controller.go -destination=mock/bg2_controller.go
type Manager interface {
	InitDomain(context.Context, domain.BGState) error
	Warmup(context.Context, domain.BGState) error
	Commit(context.Context, domain.BGState) error
	Promote(context.Context, domain.BGState) error
	Rollback(context.Context, domain.BGState) error
	DestroyDomain(context.Context, domain.BGNamespaces) error
	ListDomains(context.Context) ([]domain.BGNamespaces, error)
}

func NewController(bgManager Manager) *Controller {
	return &Controller{bgManager}
}

func (c *Controller) InitDomain(fiberCtx *fiber.Ctx, body *BGStateOperation) error {
	return handleRequestWithBody(fiberCtx, "Init", func() error {
		return c.bgManager.InitDomain(fiberCtx.UserContext(), *body.BGState)
	})
}

func (c *Controller) Warmup(fiberCtx *fiber.Ctx, body *BGStateOperation) error {
	return handleRequestWithBody(fiberCtx, "Warmup", func() error {
		return c.bgManager.Warmup(fiberCtx.UserContext(), *body.BGState)
	})
}

func (c *Controller) Commit(fiberCtx *fiber.Ctx, body *BGStateOperation) error {
	return handleRequestWithBody(fiberCtx, "Commit", func() error {
		return c.bgManager.Commit(fiberCtx.UserContext(), *body.BGState)
	})
}

func (c *Controller) Promote(fiberCtx *fiber.Ctx, body *BGStateOperation) error {
	return handleRequestWithBody(fiberCtx, "Promote", func() error {
		return c.bgManager.Promote(fiberCtx.UserContext(), *body.BGState)
	})
}

func (c *Controller) Rollback(fiberCtx *fiber.Ctx, body *BGStateOperation) error {
	return handleRequestWithBody(fiberCtx, "Rollback", func() error {
		return c.bgManager.Rollback(fiberCtx.UserContext(), *body.BGState)
	})
}

func (c *Controller) DestroyDomain(fiberCtx *fiber.Ctx, body *domain.BGNamespaces) error {
	return handleRequestWithBody(fiberCtx, "Destroy Domain", func() error {
		return c.bgManager.DestroyDomain(fiberCtx.UserContext(), *body)
	})
}

func (c *Controller) ListDomains(ctx *fiber.Ctx) error {
	if list, err := c.bgManager.ListDomains(ctx.UserContext()); err == nil {
		return controller.RespondWithJson(ctx, http.StatusOK, list)
	} else {
		return err
	}
}

func handleRequestWithBody(fiberCtx *fiber.Ctx, operationName string, handler func() error) error {
	ctx := fiberCtx.UserContext()
	log.InfoC(ctx, "Received request to %v: %+v", operationName, model.RequestContextOf(ctx))

	if handler != nil {
		err := handler()
		switch {
		case errors.Is(err, rabbit_service.ErrVersEntitiesExistsDuringWarmup):
			return utils.LogError(log, ctx, "rabbit configuration incompatibility: %v: %w", err.Error(), msg.Conflict)
		case err != nil:
			return err
		}
	}

	return controller.RespondWithJson(fiberCtx, http.StatusOK,
		&SyncResponse{
			Status:  ProcessStatusCompleted,
			Message: fmt.Sprintf("%v successfully finished", operationName),
		})
}
