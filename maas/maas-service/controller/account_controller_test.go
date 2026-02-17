package controller

import (
	"context"
	"encoding/json"
	"github.com/gofiber/fiber/v2"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/service/auth"
	mock_auth "github.com/netcracker/qubership-maas/service/auth/mock"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const (
	managerName     = "test"
	managerPassword = "my-super-duper-password"
)

func TestAccountController_SaveClientAccount(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	authService := mock_auth.NewMockAuthService(mockCtrl)

	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	c := NewAccountController(authService)
	app.Post("/test", c.CreateAccount)
	authService.EXPECT().CreateUserAccount(gomock.Any(), gomock.Any()).
		Return(true, nil).
		Times(1)
	authService.EXPECT().CreateUserAccount(gomock.Any(), gomock.Any()).
		Return(false, nil).
		Times(1)

	req := httptest.NewRequest("POST", "/test", strings.NewReader(`{
			"username": "client",
			"password": "client",
			"namespace": "_GLOBAL",
			"roles": ["manager", "agent"]
		}`))
	{
		resp, err := app.Test(req, 100)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
		assert.NotNil(t, resp.Body)
	}

	{
		resp, err := app.Test(req, 100)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotNil(t, resp.Body)
	}
}

func TestAccountController_UpdatePassword(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {

		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		authService := auth.NewAuthService(auth.NewAuthDao(baseDao), nil, nil, nil)
		accountController := NewAccountController(authService)

		_, err := authService.CreateNewManager(ctx, &model.ManagerAccountDto{
			Username: managerName,
			Password: managerPassword,
		})
		assert.NoError(t, err)

		app.Put("/auth/account/manager/:name/password", SecurityMiddleware([]model.RoleName{model.ManagerRole}, authService.IsAccessGrantedWithBasic, nil), accountController.UpdatePassword)
		testUpdatePasswordPath := "/auth/account/manager/" + managerName + "/password"

		req := httptest.NewRequest("PUT", testUpdatePasswordPath, nil)
		req.SetBasicAuth(managerName, managerPassword)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		req = httptest.NewRequest("PUT", "/auth/account/manager/not-manager/password", strings.NewReader("new-"+managerPassword))
		req.SetBasicAuth(managerName, managerPassword)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		req = httptest.NewRequest("PUT", testUpdatePasswordPath, nil)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		req = httptest.NewRequest("PUT", testUpdatePasswordPath, strings.NewReader("new-"+managerPassword))
		req.SetBasicAuth(managerName, managerPassword)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		req = httptest.NewRequest("PUT", testUpdatePasswordPath, strings.NewReader("new-"+managerPassword))
		req.SetBasicAuth(managerName, managerPassword)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		req = httptest.NewRequest("PUT", testUpdatePasswordPath, strings.NewReader("new-"+managerPassword))
		req.SetBasicAuth(managerName, "new-"+managerPassword)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func TestAccountController_DeleteClientAccount(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {

		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		authService := auth.NewAuthService(auth.NewAuthDao(baseDao), nil, nil, nil)
		accountController := NewAccountController(authService)

		_, err := authService.CreateNewManager(ctx, &model.ManagerAccountDto{
			Username: managerName,
			Password: managerPassword,
		})
		assert.NoError(t, err)

		testClienetUsername := "test-client"
		clientAccount := model.ClientAccountDto{
			Username:  testClienetUsername,
			Password:  "test-client-password",
			Namespace: "test-namespace",
			Roles:     []model.RoleName{"test-role"},
		}

		_, err = authService.CreateUserAccount(ctx, &clientAccount)
		assert.NoError(t, err)

		account, err := authService.GetAccountByUsername(ctx, testClienetUsername)
		assert.NoError(t, err)
		assert.Equal(t, testClienetUsername, account.Username)

		app.Delete("/test", SecurityMiddleware([]model.RoleName{model.ManagerRole}, authService.IsAccessGrantedWithBasic, nil), accountController.DeleteClientAccount)

		userAccountJson, err := json.Marshal(clientAccount)
		assert.NoError(t, err)

		req := httptest.NewRequest("DELETE", "/test", strings.NewReader(string(userAccountJson)))
		req.SetBasicAuth(managerName, managerPassword)

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)

		account, err = authService.GetAccountByUsername(ctx, testClienetUsername)
		assert.NoError(t, err)
		assert.Nil(t, account)

		// respond with "No Content" even if no account found
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})
}
