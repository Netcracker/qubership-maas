package v1

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-maas/controller"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/cr"
	"net/http"
	"strconv"
)

type CustomResourceController struct {
	customResourceService CustomResourceProcessorService
}

//go:generate mockgen -source=custom_resource.go -destination=custom_resource_mock.go -package v1
type CustomResourceProcessorService interface {
	Apply(ctx context.Context, customResource *cr.CustomResourceRequest, action cr.Action) (*cr.CustomResourceWaitEntity, error)
	GetStatus(ctx context.Context, trackingId int64) (*cr.CustomResourceWaitEntity, error)
	Terminate(ctx context.Context, trackingId int64) (*cr.CustomResourceWaitEntity, error)
}

func NewCustomResourceController(customResourceService CustomResourceProcessorService) *CustomResourceController {
	return &CustomResourceController{customResourceService: customResourceService}
}

func (c *CustomResourceController) Create(fiberCtx *fiber.Ctx, customResourceRequest *cr.CustomResourceRequest) error {
	return c.apply(fiberCtx, customResourceRequest, cr.ActionCreate)
}

func (c *CustomResourceController) Delete(fiberCtx *fiber.Ctx, customResourceRequest *cr.CustomResourceRequest) error {
	return c.apply(fiberCtx, customResourceRequest, cr.ActionDelete)
}

func (c *CustomResourceController) apply(fiberCtx *fiber.Ctx, customResourceRequest *cr.CustomResourceRequest, action cr.Action) error {
	waitEntity, err := c.customResourceService.Apply(fiberCtx.UserContext(), customResourceRequest, action)
	if err != nil {
		return fmt.Errorf("failed to apply custom resource request: %w", err)
	}
	if waitEntity != nil {
		return controller.RespondWithJson(fiberCtx, http.StatusAccepted, cr.CustomResourceResponse{
			Status:     waitEntity.Status,
			TrackingId: waitEntity.TrackingId,
			Conditions: createCustomResourceConditions(waitEntity),
		})
	}

	return controller.RespondWithJson(fiberCtx, http.StatusOK, cr.OkCRResponse)
}

func (c *CustomResourceController) Status(fiberCtx *fiber.Ctx) error {
	trackingIdParam := fiberCtx.Params("trackingId")
	trackingId, err := strconv.ParseInt(trackingIdParam, 10, 64)
	if err != nil {
		return fmt.Errorf("trackingId '%s' must be integer value: %s: %w", trackingIdParam, err.Error(), msg.BadRequest)
	}
	waitEntity, err := c.customResourceService.GetStatus(fiberCtx.UserContext(), trackingId)
	if err != nil {
		return err
	}
	if waitEntity == nil {
		return controller.Respond(fiberCtx, http.StatusNotFound)
	}
	return controller.RespondWithJson(fiberCtx, http.StatusOK, cr.CustomResourceResponse{
		Status:     waitEntity.Status,
		TrackingId: waitEntity.TrackingId,
		Conditions: createCustomResourceConditions(waitEntity),
	})
}

func (c *CustomResourceController) Terminate(fiberCtx *fiber.Ctx) error {
	trackingIdParam := fiberCtx.Params("trackingId")
	trackingId, err := strconv.ParseInt(trackingIdParam, 10, 64)
	if err != nil {
		return fmt.Errorf("trackingId '%s' must be integer value: %s: %w", trackingIdParam, err.Error(), msg.BadRequest)
	}
	waitEntity, err := c.customResourceService.Terminate(fiberCtx.UserContext(), trackingId)
	if err != nil {
		return err
	}
	if waitEntity == nil {
		return controller.Respond(fiberCtx, http.StatusNotFound)
	}
	return controller.RespondWithJson(fiberCtx, http.StatusOK, cr.CustomResourceResponse{
		Status:     waitEntity.Status,
		TrackingId: waitEntity.TrackingId,
		Conditions: createCustomResourceConditions(waitEntity),
	})
}

func createCustomResourceConditions(waitEntity *cr.CustomResourceWaitEntity) []cr.CustomResourceConditions {
	return []cr.CustomResourceConditions{
		{
			Type:  cr.CustomResourceTypeValidated,
			State: cr.CustomResourceStatusCompleted,
		},
		{
			Type:    cr.CustomResourceTypeDependenciesResolved,
			State:   waitEntity.Status,
			Reason:  "Kafka topic depends on topic template, waiting for this dependency resolving",
			Message: waitEntity.Reason,
		},
		{
			Type:   cr.CustomResourceTypeCreated,
			State:  cr.CustomResourceStatusNotStarted,
			Reason: "waiting for dependency",
		},
	}
}
