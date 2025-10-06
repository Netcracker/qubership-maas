package controller

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/netcracker/qubership-maas/validator"
	"net/http"
)

type VHostController struct {
	service     rabbit_service.RabbitService
	authService auth.AuthService
}

type VhostAndConfigResult struct {
	VHost    model.VHostRegistrationResponse `json:"vhost"`
	Entities *model.RabbitEntities           `json:"entities,omitempty"`
}

var vLog logging.Logger

func init() {
	vLog = logging.GetLogger("vhost-controller")

}

func NewVHostController(s rabbit_service.RabbitService, a auth.AuthService) *VHostController {
	return &VHostController{s, a}
}

// @Summary Get Or Create VHost using Agent Role
// @Description Get Or Create VHost
// @ID GetOrCreateVHost
// @Tags V1
// @Param request body model.VHostRegistrationReqDto true "VHostRegistrationReqDto"
// @Param X-Origin-Namespace header string true "cloudbss311-platform-core-support-dev3"
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {object}    model.VHostRegistrationResponse
// @Success 201 {object}    model.VHostRegistrationResponse
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 403 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/rabbit/vhost [post]
func (c *VHostController) GetOrCreateVHost(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	var vhostRequest model.VHostRegistrationReqDto
	if err := json.Unmarshal(fiberCtx.Body(), &vhostRequest); err != nil {
		return utils.LogError(log, ctx, "failed to read vhost registration request body: %s: %w", err.Error(), msg.BadRequest)
	}

	if err := validator.Get().Struct(&vhostRequest); err != nil {
		return utils.LogError(log, ctx, "error in CheckFormat of classifier during GetOrCreateVHost: %s: %w", err.Error(), msg.BadRequest)
	}

	namespace := model.RequestContextOf(ctx).Namespace
	vLog.InfoC(ctx, "Received request to create vhost in namespace: %v: %+v", namespace, vhostRequest)

	found, vHostRegistration, err := c.service.GetOrCreateVhost(ctx, vhostRequest.Instance, &vhostRequest.Classifier, nil)
	if err != nil {
		switch {
		case errors.Is(err, rabbit_service.ErrNoDefaultRabbit):
			return utils.LogError(log, ctx, "virtual host was not created because default RabbitMQ instance is not configured: %w", msg.Conflict)

		case errors.Is(err, dao.ClassifierUniqIndexErr):
			return utils.LogError(log, ctx, "failed to save virtual host with classifier %+v and namespace %s because of unique constraint", vhostRequest.Classifier, namespace)
		default:
			return utils.LogError(log, ctx, "error during vhost getting or creation: %s", err.Error())
		}
	}
	extended := fiberCtx.QueryBool("extended", false)
	cnnUrl, err := c.service.GetConnectionUrl(ctx, vHostRegistration)
	if err != nil {
		return utils.LogError(log, ctx, "error during GetConnectionUrl in GetOrCreateVHost: %s", err.Error())
	}
	response := model.VHostRegistrationResponse{
		Cnn:      cnnUrl,
		Username: vHostRegistration.User,
		Password: vHostRegistration.Password,
	}

	if extended {
		apiUrl, err := c.service.GetApiUrl(ctx, vHostRegistration.InstanceId)
		if err != nil {
			return utils.LogError(log, ctx, "error during GetApiUrl in GetOrCreateVHost: %s", err.Error())
		}
		response.ApiUrl = apiUrl
	}
	if found {
		vLog.InfoC(ctx, "Found existing vhost registration: %+v", vHostRegistration)
		return RespondWithJson(fiberCtx, http.StatusOK, response)
	} else {
		vLog.InfoC(ctx, "Created new vhost registration: %+v", vHostRegistration)
		return RespondWithJson(fiberCtx, http.StatusCreated, response)
	}
}

func (c *VHostController) SearchVhosts(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	tLog.InfoC(ctx, "Received request to search vhosts by criteria %s, ctx: %+v", string(fiberCtx.Body()), model.RequestContextOf(ctx))

	searchForm, err := model.ConvertToSearchForm(string(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "error during converting to search form while in SearchVhosts: %s", err.Error())
	}

	vhosts := make([]model.VHostRegistration, 0)

	vhosts, err = c.service.FindVhostWithSearchForm(ctx, searchForm)
	if err != nil {
		return utils.LogError(log, ctx, "Error occurred while searching for vhosts in database: '%w'", err)
	}

	if vhosts == nil {
		vhosts = make([]model.VHostRegistration, 0)
	}

	return RespondWithJson(fiberCtx, http.StatusOK, vhosts)
}

// @Summary Delete VHost using Agent Role
// @Description Delete VHost
// @ID DeleteVHost
// @Tags V1
// @Param request body model.SearchForm  true "SearchForm"
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Produce  json
// @Security BasicAuth[agent]
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 204
// @Router /api/v1/rabbit/vhost [delete]
func (c *VHostController) DeleteVHost(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	searchForm, err := model.ConvertToSearchForm(string(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "error during converting to search form: %s", err.Error())
	}

	err = c.service.RemoveVHosts(ctx, searchForm, model.RequestContextOf(ctx).Namespace)
	if err != nil {
		return err
	}
	return RespondWithJson(fiberCtx, http.StatusNoContent, nil)
}

// @Summary Get VHost And Config By Classifier using Agent Role
// @Description Set Default Rabbit Instance
// @ID GetVHostAndConfigByClassifier
// @Tags V1
// @Param request body  model.Classifier true "Classifier"
// @Param X-Origin-Namespace header string true "cloudbss311-platform-core-support-dev3"
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200 {object}    VhostAndConfigResult
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Failure 204 {object}	map[string]string
// @Router /api/v1/rabbit/vhost/get-by-classifier [post]
func (c *VHostController) GetVHostAndConfigByClassifier(fiberCtx *fiber.Ctx) error {
	return ExtractAndValidateClassifier(fiberCtx, func(ctx context.Context, classifier *model.Classifier) error {
		result := &VhostAndConfigResult{}

		vHostRegistration, err := c.service.FindVhostByClassifier(ctx, classifier)
		if err != nil {
			return utils.LogError(log, ctx, "error while searching virtual host by classifier '%+v': %w ", classifier, err)
		}
		if vHostRegistration == nil {
			return utils.LogError(log, ctx, "virtual host with classifier %+v was not found: %w", *classifier, msg.NotFound)
		}

		entities, err := c.service.GetConfig(ctx, *classifier)
		if err != nil {
			return utils.LogError(log, ctx, "error while getting virtual host's configs by classifier %+v: %w", classifier, err)
		}

		extended := fiberCtx.QueryBool("extended", false)
		cnnUrl, err := c.service.GetConnectionUrl(ctx, vHostRegistration)
		if err != nil {
			return utils.LogError(log, ctx, "error during GetConnectionUrl in GetVHostAndConfigByClassifier: %w", err)
		}

		result.VHost = model.VHostRegistrationResponse{
			Cnn:      cnnUrl,
			Username: vHostRegistration.User,
			Password: vHostRegistration.Password,
		}
		if extended {
			apiUrl, err := c.service.GetApiUrl(ctx, vHostRegistration.InstanceId)
			if err != nil {
				return utils.LogError(log, ctx, "error during GetApiUrl in GetOrCreateVHost: %s", err.Error())
			}
			result.VHost.ApiUrl = apiUrl
		}
		result.Entities = entities
		return RespondWithJson(fiberCtx, http.StatusOK, result)
	})
}

// @Summary Validate Rabbit Configs using Agent Role
// @Description Validate Rabbit Configs
// @ID ValidateRabbitConfigs
// @Tags V2
// @Produce  json
// @Security BasicAuth[agent]
// @Param X-Origin-Namespace header string   true "X-Origin-Namespace"
// @Success 200 {object}	 map[string][]model.LazyBindingDto
// @Failure 500 {object}	map[string]string
// @Failure 400 {object}	map[string]string
// @Router /api/v2/rabbit/validations [get]
// Namespace parameter shouldn't be emoty
func (c *VHostController) ValidateRabbitConfigs(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := model.RequestContextOf(ctx).Namespace
	lazyBindings, err := c.service.GetLazyBindings(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error searching lazy bindings by namespace '%v': %w", namespace, err)
	}
	log.InfoC(ctx, "Lazy bindings for namespace '%v': '%+v'", namespace, lazyBindings)

	//now we show only bindings without exchanges, but then it may be extended
	responseMap := map[string][]model.LazyBindingDto{"bindings": lazyBindings}
	return RespondWithJson(fiberCtx, http.StatusOK, responseMap)
}

// @Summary Recover Namespace using Agent Role
// @Description Recover Namespace
// @ID RecoverNamespace
// @Tags V2
// @Param X-Origin-Namespace header string true "X-Origin-Namespace"
// @Param namespace path string true "namespace"
// @Produce  json
// @Security BasicAuth[agent]
// @Success 200
// @Failure 500 {object}	map[string]string
// @Router /api/v2/rabbit/recovery/{namespace} [post]
func (c *VHostController) RecoverNamespace(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	namespace := fiberCtx.Params("namespace")
	if namespace == "" {
		log.WarnC(ctx, "parameter 'namespace' is empty, use header namespace instead")
		namespace = model.RequestContextOf(ctx).Namespace
	}

	err := c.service.RecoverVhostsByNamespace(ctx, namespace)
	if err != nil {
		return utils.LogError(log, ctx, "error in RecoverVhostsByNamespace while in RecoverNamespace in vhost controller: %s", err.Error())
	}

	return RespondWithJson(fiberCtx, http.StatusOK, nil)
}

func (c *VHostController) RotatePasswords(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	searchForm, err := model.ConvertToSearchForm(string(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "error during converting to search form while in RotatePasswords: %w", err)
	}

	vhosts, err := c.service.RotatePasswords(ctx, searchForm)
	if err != nil {
		return utils.LogError(log, ctx, "error during RotatePasswords: %w", err)
	}

	return RespondWithJson(fiberCtx, http.StatusOK, vhosts)
}
