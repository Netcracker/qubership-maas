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
	"github.com/netcracker/qubership-maas/utils"
	"net/http"
)

type AccountController struct {
	service auth.AuthService
}

var aLog logging.Logger

func init() {
	aLog = logging.GetLogger("auth-controller")
}

func NewAccountController(s auth.AuthService) *AccountController {
	return &AccountController{service: s}
}

// @Summary Create Account
// @Description Create new account
// @ID CreateAccount
// @Tags V1
// @Produce  json
// @Param	request 		body     model.ClientAccountDto  true   "ClientAccountDto"
// @Security BasicAuth[manager]
// @Success 200 {object}	map[string]string
// @Success 201 {object}	model.ClientAccountDto
// @Failure 400 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Router /api/v1/auth/account/client [post]
func (a *AccountController) CreateAccount(fiberCtx *fiber.Ctx) error {
	var account model.ClientAccountDto
	return UnmarshalRequestBody(fiberCtx, &account, func(ctx context.Context) error {
		aLog.InfoC(ctx, "Create new auth account: '%+v'", account)
		isNew, err := a.service.CreateUserAccount(ctx, &account)
		if err != nil {
			return utils.LogError(log, ctx, "error create account %+v: %w", account, err)
		}

		return RespondWithJson(fiberCtx, utils.Iff(isNew, http.StatusCreated, http.StatusOK), account)
	})
}

// @Summary Delete Client Account using Manager Role
// @Description Delete Client Account
// @ID DeleteClientAccount
// @Tags V1
// @Produce  json
// @Param	request 		body     model.ClientAccountDto  true   "ClientAccountDto"
// @Security BasicAuth[manager]
// @Success 204
// @Failure 400 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Router /api/v1/auth/account/client [delete]
func (a *AccountController) DeleteClientAccount(fiberCtx *fiber.Ctx) error {
	var accountRequest model.ClientAccountDto
	return UnmarshalRequestBody(fiberCtx, &accountRequest, func(ctx context.Context) error {
		aLog.InfoC(ctx, "Get request to delete account")
		err := a.service.DeleteUserAccount(ctx, accountRequest.Username)
		if errors.Is(err, msg.NotFound) {
			return RespondWithJson(fiberCtx, http.StatusNoContent, "")
		}
		if err != nil {
			return utils.LogError(log, ctx, "error delete account: %w", err)
		}

		aLog.InfoC(ctx, "User account '%+v' was deleted successfully", accountRequest)
		return RespondWithJson(fiberCtx, http.StatusNoContent, "")
	})

}

// @Summary Save Manager Account using Manager Role
// @Description Save Manager Account
// @ID SaveManagerAccount
// @Tags V1
// @Produce  json
// @Param	request 		body     model.ManagerAccountDto  true   "ManagerAccountDto"
// @Security BasicAuth[manager]
// @Success 201 {object}	model.ManagerAccountDto
// @Failure 400 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Failure 403 {object}	map[string]string
// @Failure 409 {object}	map[string]string
// @Router /api/v1/auth/account/manager [post]
// TODO too much logic for controller, move to service and use global transaction to prevent race between check and create
func (a *AccountController) SaveManagerAccount(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	log.InfoC(ctx, "Get request on create a new manager account")

	var accountRequest model.ManagerAccountDto
	if err := json.Unmarshal(fiberCtx.Body(), &accountRequest); err != nil {
		return utils.LogError(log, ctx, "error decode request body: %v: %w", err.Error(), msg.BadRequest)
	}

	username, password, err := utils.GetBasicAuth(fiberCtx)
	if err != nil {
		log.InfoC(ctx, "Received request on create first manager account")
		isFirst, err := a.service.IsFirstAccountManager(ctx)
		if err != nil {
			return utils.LogError(log, ctx, "error checking service manager account presence: %w", err)
		}
		if !isFirst {
			return utils.LogError(log, ctx, "at least one account manager already exists and you need to specify account manager credentials: %w", msg.AuthError)
		}

		if managerAccount, err := a.service.CreateNewManager(ctx, &accountRequest); err != nil {
			return utils.LogError(log, ctx, "couldn't create account: %w", err)
		} else {
			log.InfoC(ctx, "First manager account was created successfully")
			return RespondWithJson(fiberCtx, http.StatusCreated, managerAccount)
		}
	}

	log.InfoC(ctx, "Received request on create one more manage account")
	if _, err := a.service.IsAccessGranted(ctx, username, password, model.ManagerAccountNamespace, []model.RoleName{model.ManagerRole}); err != nil {
		return utils.LogError(log, ctx, "password verification does not passed: %w", err)
	}
	if managerAccount, err := a.service.CreateNewManager(ctx, &accountRequest); err != nil {
		return utils.LogError(log, ctx, "couldn't create account: %w", err)
	} else {
		log.InfoC(ctx, "one more manager account was created successfully")
		return RespondWithJson(fiberCtx, http.StatusCreated, managerAccount)
	}
}

// @Summary Get All Accounts using Manager Role
// @Description Get All Accounts
// @ID GetAllAccounts
// @Tags V1
// @Produce  json
// @Security BasicAuth[manager]
// @Success 200 {array}	    model.Account
// @Failure 404 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Router /api/v1/auth/accounts [get]
func (a *AccountController) GetAllAccounts(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	tLog.InfoC(ctx, "Received request to get all accounts")
	accounts, err := a.service.GetAllAccounts(ctx)
	if err != nil {
		return err
	}
	return RespondWithJson(fiberCtx, http.StatusOK, accounts)
}

// @Summary Update Password using Manager Role
// @Description Update Password
// @ID UpdatePassword
// @Tags V2
// @Produce  json
// @Param	name 		path     string  true   "new-password"
// @Security BasicAuth[manager]
// @Success 200
// @Failure 400 {object}    map[string]string
// @Failure 500 {object}	map[string]string
// @Failure 404 {object}	map[string]string
// @Router /api/v2/auth/account/manager/{name}/password [put]
func (a *AccountController) UpdatePassword(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	name := fiberCtx.Params("name")
	tLog.InfoC(ctx, "Received request to update '%s' password", name)
	err := a.service.UpdateUserPassword(ctx, name, utils.SecretString(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "error update password: %w", err)
	}

	tLog.InfoC(ctx, "Password successfully updated for `%v'", name)
	return RespondWithJson(fiberCtx, http.StatusOK, nil)
}
