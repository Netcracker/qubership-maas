package controller

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/netcracker/qubership-core-lib-go/v3/security/tokenverifier"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

type mockTokenVerifier struct {
	token     string
	username  string
	namespace string
}

func (mv mockTokenVerifier) Verify(ctx context.Context, token string) (*tokenverifier.Claims, error) {
	if token != mv.token {
		return nil, errors.Join(errors.New("invalid token"), msg.AuthError)
	}
	return &tokenverifier.Claims{
		Kubernetes: tokenverifier.K8sClaims{
			Namespace: mv.namespace,
			ServiceAccount: tokenverifier.ServiceAccountClaim{
				Name: mv.username,
			},
		},
	}, nil
}

func TestSecurityMiddleware_Anonymous(t *testing.T) {
	testRoleName := model.AgentRole
	testNamespaceName := "test-namespace"
	validToken := "valid_token"
	tokenVerifier := mockTokenVerifier{
		token:     validToken,
		username:  "test-service",
		namespace: testNamespaceName,
	}
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {

		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		authService := auth.NewAuthService(auth.NewAuthDao(baseDao), nil, nil, tokenVerifier)

		_, err := authService.CreateUserAccount(ctx, &model.ClientAccountDto{
			Username:  "client",
			Password:  "client",
			Roles:     []model.RoleName{testRoleName},
			Namespace: testNamespaceName,
		})
		assert.NoError(t, err)

		app.Get("/not-anonymous", SecurityMiddleware([]model.RoleName{testRoleName}, authService.IsAccessGrantedWithBasic, authService.IsAccessGrantedWithToken), func(ctx *fiber.Ctx) error {
			return ctx.Status(200).JSON("ok")
		})

		req := httptest.NewRequest("GET", "/not-anonymous", nil)
		resp, err := app.Test(req)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		req = httptest.NewRequest("GET", "/not-anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		req.SetBasicAuth("client", "client")
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		req = httptest.NewRequest("GET", "/not-anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		setBrearerAuth(req, validToken)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		app.Get("/anonymous", SecurityMiddleware([]model.RoleName{model.AnonymousRole, testRoleName}, authService.IsAccessGrantedWithBasic, authService.IsAccessGrantedWithToken), func(ctx *fiber.Ctx) error {
			return ctx.Status(200).JSON("ok")
		})

		req = httptest.NewRequest("GET", "/anonymous", nil)
		resp, err = app.Test(req)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		req = httptest.NewRequest("GET", "/anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		req.SetBasicAuth("client", "client")
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		req = httptest.NewRequest("GET", "/anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		setBrearerAuth(req, validToken)
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		req = httptest.NewRequest("GET", "/anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		req.SetBasicAuth("wrong-client", "wrong-client")
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)

		req = httptest.NewRequest("GET", "/anonymous", nil)
		req.Header.Add(HeaderXNamespace, testNamespaceName)
		setBrearerAuth(req, "invalid_token")
		resp, err = app.Test(req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
	})
}

func setBrearerAuth(r *http.Request, token string) {
	r.Header.Add(fiber.HeaderAuthorization, "Bearer "+token)
}

func TestTmfErrorHandler_ErrorFormat(t *testing.T) {
	app := fiber.New(fiber.Config{ErrorHandler: TmfErrorHandler})
	app.Get("/error", func(ctx *fiber.Ctx) error {
		return fmt.Errorf("test error: %w", msg.NotFound)
	})

	req := httptest.NewRequest("GET", "/error", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	msg, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	var tmfError TmfError
	err = json.Unmarshal(msg, &tmfError)
	assert.NoError(t, err)
	assert.NotEmpty(t, tmfError.Id)
	assert.Equal(t, strconv.Itoa(http.StatusNotFound), tmfError.Status)
	assert.Equal(t, "not found", tmfError.Reason)
	assert.Equal(t, "test error: not found", tmfError.Message)
	assert.Equal(t, "MAAS-0600", tmfError.Code)
	assert.Equal(t, "NC.TMFErrorResponse.v1.0", tmfError.NSType)
}

func Test_maskPasswordInBody(t *testing.T) {
	body := `{
		                    "username": "maas-agent-sfa-dev-upgrade",
		                    "password": "v57tt5t17m",
		                    "namespace": "sfa-dev-upgrade",
		                    "roles": [
		                        "agent"
		                    ]
		               }`
	assert.Regexp(t, regexp.MustCompile(`"password": "[*]{6}"`), maskPasswordInBody(body))
}

type testDto struct {
	First  string
	Second map[string]string
}

func TestWithBody(t *testing.T) {
	app := fiber.New()
	app.Get("/json", WithBody(json.Unmarshal, func(ctx *fiber.Ctx, body *testDto) error {
		assert.Equal(t, "firstVal", body.First)
		assert.Equal(t, "v1", body.Second["k1"])
		assert.Equal(t, "v2", body.Second["k2"])
		return nil
	}))
	req := httptest.NewRequest("GET", "/json", strings.NewReader(`{"first": "firstVal", "second": {"k1": "v1", "k2": "v2"}}`))
	_, err := app.Test(req)
	assert.NoError(t, err)

	app.Get("/yaml", WithBody(yaml.Unmarshal, func(ctx *fiber.Ctx, body *testDto) error {
		assert.Equal(t, "firstVal", body.First)
		assert.Equal(t, "v1", body.Second["k1"])
		assert.Equal(t, "v2", body.Second["k2"])
		return nil
	}))
	req = httptest.NewRequest("GET", "/yaml", strings.NewReader(`{"first": "firstVal", "second": {"k1": "v1", "k2": "v2"}}`))
	_, err = app.Test(req)
	assert.NoError(t, err)
}

func TestFallbackCrApiVersion(t *testing.T) {
	testData := []struct {
		config      string
		input       string
		expectation string
	}{
		{
			config:      "old.api.version/v1",
			input:       `{"apiVersion": "old.api.version/v1", "kind": "Test"}`,
			expectation: `{"apiVersion": "core.qubership.org/v1", "kind": "Test"}`,
		},
		{
			config:      "",
			input:       `{"apiVersion": "abc.qubership.org/v1", "kind": "Test"}`,
			expectation: `{"apiVersion": "abc.qubership.org/v1", "kind": "Test"}`,
		},
	}

	for _, tt := range testData {
		app := fiber.New()
		app.Post("/", FallbackCrApiVersion(tt.config), func(c *fiber.Ctx) error {
			return c.Send(c.Body())
		})

		req := httptest.NewRequest("POST", "/", strings.NewReader(tt.input))
		resp, err := app.Test(req)
		assert.NoError(t, err)

		body, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, tt.expectation, string(body))
	}
}
