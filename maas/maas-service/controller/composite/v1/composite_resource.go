package v1

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-maas/controller"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/composite"
	"golang.org/x/exp/slices"
)

type RegistrationController struct {
	registrationService RegistrationService
}

//go:generate mockgen -source=composite_resource.go -destination=composite_resource_mock.go -package v1
type RegistrationService interface {
	Upsert(ctx context.Context, registrationRequest *composite.CompositeRegistration) error
	GetByBaseline(ctx context.Context, baseline string) (*composite.CompositeRegistration, error)
	List(ctx context.Context) ([]composite.CompositeRegistration, error)
	Destroy(ctx context.Context, namespace string) error
}

func NewRegistrationController(registrationService RegistrationService) *RegistrationController {
	return &RegistrationController{registrationService: registrationService}
}

// @Summary Create Composite Registration
// @Description Register new Composite
// @ID Create
// @Tags V1
// @Accept  json
// @Produce  json
// @Param	request 		body     v1.RegistrationRequest  true   "RegistrationRequest"
// @Security BasicAuth[agent]
// @Success 204
// @Failure 409 {object}	map[string]string
// @Failure 500 {object}	map[string]string
// @Router /api/composite/v1/structure [post]
func (c *RegistrationController) Create(fiberCtx *fiber.Ctx, registrationRequest *RegistrationRequest) error {
	if !slices.Contains(registrationRequest.Namespaces, registrationRequest.Id) {
		return fmt.Errorf("'namespaces' array MUST contain namespace from 'id' param: %w", msg.BadRequest)
	}
	err := c.registrationService.Upsert(fiberCtx.UserContext(), registrationRequest.ToCompositeRegistration())
	if err != nil {
		return err
	}
	return controller.Respond(fiberCtx, http.StatusNoContent)
}

// @Summary Get Composite Registration By ID
// @Description Get Composite Registration By Baseline ID
// @ID GetById
// @Tags V1
// @Produce  json
// @Param	id 		path     string  true   "Registration ID"
// @Security BasicAuth[agent]
// @Success 200 {object} v1.RegistrationResponse
// @Failure 404
// @Failure 500 {object} map[string]string
// @Router /api/composite/v1/structure/{id} [get]
func (c *RegistrationController) GetById(fiberCtx *fiber.Ctx) error {
	baseline := fiberCtx.Params("id")
	registration, err := c.registrationService.GetByBaseline(fiberCtx.UserContext(), baseline)
	if err != nil {
		return err
	}
	if registration == nil {
		return fmt.Errorf("No composite registration found by compositeId: %v: %w", baseline, msg.NotFound)
	}

	return controller.RespondWithJson(fiberCtx, http.StatusOK, NewRegistrationResponse(registration))
}

// @Summary Get All Composite Registrations
// @Description Get All Composite Registrations
// @ID GetAll
// @Tags V1
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {object} []v1.RegistrationResponse
// @Failure 500 {object}	map[string]string
// @Router /api/composite/v1/structures [get]
func (c *RegistrationController) GetAll(fiberCtx *fiber.Ctx) error {
	entites, err := c.registrationService.List(fiberCtx.UserContext())
	if err != nil {
		return err
	}

	response := make([]RegistrationResponse, 0)
	for _, entity := range entites {
		response = append(response, RegistrationResponse(entity))
	}

	return controller.RespondWithJson(fiberCtx, http.StatusOK, response)
}

// @Summary Destroy Composite Registrations
// @Description Destroy Composite Registrations
// @ID DeleteById
// @Tags V1
// @Param	id 		path     string  true   "Registration ID"
// @Produce  json
// @Security BasicAuth[agent]
// @Success 204
// @Failure 404
// @Failure 500 {object}	map[string]string
// @Router /api/composite/v1/structure/{id} [delete]
func (c *RegistrationController) DeleteById(fiberCtx *fiber.Ctx) error {
	baseline := fiberCtx.Params("id")
	err := c.registrationService.Destroy(fiberCtx.UserContext(), baseline)
	if err != nil {
		return err
	}
	return controller.Respond(fiberCtx, http.StatusNoContent)
}
