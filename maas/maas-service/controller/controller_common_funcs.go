package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/kubernetes/oidc"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
	v "github.com/netcracker/qubership-maas/validator"
	"golang.org/x/exp/slices"
	"gopkg.in/yaml.v3"
)

const (
	HeaderXNamespace    = "X-Origin-Namespace"
	HeaderXMicroservice = "X-Origin-Microservice"
	HeaderXVersion      = "X-Version"
	HeaderXRequestId    = "X-Request-Id"

	HeaderXCompositeIsolationDisabled = "X-Composite-Isolation"
)

type (
	Handler func(context.Context, string)
)

type ErrorResponse struct {
	Message string
}

type TmfError struct {
	Id      string            `json:"id"`
	Status  string            `json:"status,omitempty"`
	Code    string            `json:"code"`
	Reason  string            `json:"reason"`
	Message string            `json:"message,omitempty"`
	Meta    map[string]string `json:"meta,omitempty"`
	NSType  string            `json:"@type"`
}

var log logging.Logger
var passwordRegExp = regexp.MustCompile(`"password": "[^[:space:]]*",`)

func init() {
	log = logging.GetLogger("controller")
}

type RequestBodyHandler func(ctx context.Context) (interface{}, error)

type authorizeFunc func(context.Context, string, utils.SecretString, string, []model.RoleName) (*model.Account, error)
type authorizeWithTokenFunc func(context.Context, oidc.Verifier, string, string, []model.RoleName) (*model.Account, error)

func SecurityMiddleware(roles []model.RoleName, verifier oidc.Verifier, authorize authorizeFunc, authorizeWithToken authorizeWithTokenFunc) fiber.Handler {
	return func(ctx *fiber.Ctx) error {
		userCtx := ctx.UserContext()
		var account *model.Account

		authHeader := strings.Split(string(ctx.Request().Header.Peek(fiber.HeaderAuthorization)), "")
		if len(authHeader) != 2 {
			return utils.LogError(log, userCtx, "security middleware error: invalid Authorization header")
		}
		namespace := string(ctx.Request().Header.Peek(HeaderXNamespace))

		if authHeader[0] == "Basic" {
			username, password, err := utils.GetBasicAuth(ctx)
			if err != nil {
				if slices.Contains(roles, model.AnonymousRole) {
					log.WarnC(userCtx, "Anonymous access will be dropped in future releases for: %s", ctx.OriginalURL())
					return ctx.Next()
				}
				return utils.LogError(log, userCtx, "security middleware error: %w", err)
			}

			account, err = authorize(ctx.UserContext(), username, password, namespace, roles)
			if err != nil {
				return utils.LogError(log, userCtx, "request authorization failure: %w", err)
			}
		} else if authHeader[0] == "Bearer" {
			acc, err := authorizeWithToken(ctx.Context(), verifier, authHeader[1], namespace, roles)
			if err != nil {
				return utils.LogError(log, userCtx, "request authorization failure: %w", err)
			}
			account = acc
		} else {
			return utils.LogError(log, userCtx, "security middleware error: unsupported authentication method: %s", authHeader[0])
		}

		compositeIsolationDisabled := strings.ToLower(string(ctx.Request().Header.Peek(HeaderXCompositeIsolationDisabled))) == "disabled"

		secCtx := model.NewSecurityContext(account, compositeIsolationDisabled)
		ctx.SetUserContext(model.WithSecurityContext(userCtx, secCtx))
		return ctx.Next()
	}
}

func ExtractOrAttachXRequestId(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	requestId := string(fiberCtx.Request().Header.Peek(HeaderXRequestId))
	if requestId == "" {
		requestId = uuid.New().String()
		log.InfoC(ctx, "Header 'X-Request-Id' is missing, generated requestId: %v", requestId)
	}
	ctx = context.WithValue(ctx, HeaderXRequestId, requestId)
	fiberCtx.SetUserContext(ctx)

	// add request-id to response
	fiberCtx.Set(HeaderXRequestId, requestId)

	return fiberCtx.Next()
}

func maskPasswordInBody(originalBody string) string {
	return passwordRegExp.ReplaceAllString(originalBody, `"password": "******",`)
}

func LogRequest(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	body := string(fiberCtx.Body())
	if body != "" {
		body = maskPasswordInBody(fmt.Sprintf("\n\tBody: %s", strings.Replace(body, "\n", "\n\t\t", -1)))
	}

	auth := ""
	if username, _, err := utils.GetBasicAuth(fiberCtx); err == nil && username != "" {
		auth = fmt.Sprintf(", Usr:%s", username)
	}

	namespace := string(fiberCtx.Request().Header.Peek(HeaderXNamespace))
	if namespace != "" {
		namespace = fmt.Sprintf(", Ns:%s", namespace)
	}

	log.InfoC(ctx, "Start handle request: %s %s, Src:%s%s%s%s", fiberCtx.Method(), fiberCtx.OriginalURL(), fiberCtx.IP(), namespace, auth, body)
	err := fiberCtx.Next()
	if err == nil {
		log.InfoC(ctx, "Finish serve http request")
	} else {
		fiberError := &fiber.Error{}
		if errors.As(err, &fiberError) {
			log.ErrorC(ctx, "Request processing error: code=%d message=%s", fiberError.Code, fiberError.Error())
		} else {
			log.ErrorC(ctx, "Request processing error: %s", err)
		}
	}
	return err
}

func ExtractRequestContext(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	requestContext := &model.RequestContext{}

	namespace := string(fiberCtx.Request().Header.Peek(HeaderXNamespace))
	if namespace != "" {
		requestContext.Namespace = namespace
	}

	requestContext.Microservice = string(fiberCtx.Request().Header.Peek(HeaderXMicroservice))

	rawVersion := string(fiberCtx.Request().Header.Peek(HeaderXVersion))
	version, err := model.ParseVersion(rawVersion)
	if err != nil {
		return utils.LogError(log, ctx, "request X-Version has incorrect value: %s: %w", err.Error(), msg.BadRequest)
	} else if version == "" {
		log.DebugC(ctx, "Version header is missed")
	}
	requestContext.Version = version

	ctx = model.WithRequestContext(ctx, requestContext)
	fiberCtx.SetUserContext(ctx)
	return fiberCtx.Next()
}

// Namespace parameter shouldn't be emoty
func RequiresNamespaceHeader(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()
	rc := model.RequestContextOf(ctx)
	if rc == nil {
		return errors.New("request context not yet extracted")
	}

	if rc.Namespace == "" {
		return utils.LogError(log, ctx, "request missed '%s' header: %w", HeaderXNamespace, msg.BadRequest)
	}

	return fiberCtx.Next()
}

func ParseRequestParametersFromConfig(fiberCtx *fiber.Ctx) error {
	ctx := fiberCtx.UserContext()

	narrow := func(o interface{}, out interface{}) error {
		m, err := json.Marshal(o)
		if err != nil {
			return fmt.Errorf("error narrow object: %w", err)
		}
		return json.Unmarshal(m, out)
	}

	normalizedReq, err := utils.NormalizeJsonOrYamlInput(string(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "error during NormalizeJsonOrYamlInput for initial yaml request: %s: %w", err.Error(), msg.BadRequest)
	}
	log.InfoC(ctx, "Normalized request: %v", normalizedReq)

	var objects []interface{}
	if err = json.Unmarshal([]byte(normalizedReq), &objects); err != nil {
		return utils.LogError(log, ctx, "error during unmarshalling initial yaml request %s: %w", err.Error(), msg.BadRequest)
	}

	if len(objects) != 1 {
		return utils.LogError(log, ctx, "only single aggregated YAML file is supported, not array: %w", msg.BadRequest)
	}
	object := objects[0]

	config := model.ConfigReqDto{}
	if err := narrow(object, &config); err != nil {
		return utils.LogError(log, ctx, "can't map data in parsing request parameters to: '%+v' for config: '%+v', for init object: '%+v'. error: %s: %w", model.ConfigReqDto{}, config, object, err.Error(), msg.BadRequest)
	}

	namespace := config.Spec.Namespace

	//todo check security!?
	requestContext := &model.RequestContext{}
	if namespace == "" {
		namespace = string(fiberCtx.Request().Header.Peek(HeaderXNamespace))
		if namespace == "" {
			return utils.LogError(log, ctx, "request missed namespace field in spec or in header: %w", msg.BadRequest)
		}
	}
	requestContext.Namespace = namespace

	rawVersion := config.Spec.Version
	version, err := model.ParseVersion(rawVersion)
	if err != nil {
		return utils.LogError(log, ctx, "request Version has incorrect value: %s: %w", err.Error(), msg.BadRequest)
	} else if version == model.VersionEmpty {
		log.DebugC(ctx, "Version field in spec is missed")
	}

	fiberCtx.SetUserContext(model.WithRequestContext(ctx, requestContext))
	return fiberCtx.Next()
}

func RespondWithJson(c *fiber.Ctx, code int, payload interface{}) error {
	log.InfoC(c.UserContext(), "Responding with code '%v' and payload: %+v", code, payload)
	if payload == nil {
		return c.Status(code).Send(nil)
	}
	return c.Status(code).JSON(payload)
}

func Respond(c *fiber.Ctx, code int) error {
	log.InfoC(c.UserContext(), "Responding with code '%d'", code)
	return c.Status(code).Send(nil)
}

func UnmarshalRequestBody[T any](fiberCtx *fiber.Ctx, obj T, handler func(ctx context.Context) error) error {
	ctx := fiberCtx.UserContext()
	if err := json.Unmarshal(fiberCtx.Body(), obj); err != nil {
		return utils.LogError(log, ctx, "error unmarshall body: %v: %w", err.Error(), msg.BadRequest)
	}

	return handler(ctx)
}

func TmfErrorHandler(ctx *fiber.Ctx, err error) error {
	code := http.StatusInternalServerError
	var meta map[string]string

	// bypass fiber errors to fiber default handler
	fiberErr := &fiber.Error{}
	var ve = new(validator.ValidationErrors)
	switch {
	// TODO remove in future
	case errors.As(err, ve):
		err = errors.New(strings.Join(v.FormatErrors(*ve), " "))
		code = http.StatusBadRequest
	case errors.Is(err, msg.BadRequest):
		code = http.StatusBadRequest
		meta = map[string]string{"type": "Validated"}
	case errors.Is(err, msg.NotFound):
		code = http.StatusNotFound
	case errors.Is(err, msg.Conflict):
		code = http.StatusConflict
	case errors.Is(err, msg.AuthError):
		code = http.StatusForbidden
	case errors.Is(err, msg.Gone):
		code = http.StatusGone
	case errors.Is(err, dao.DatabaseIsNotActiveError):
		code = http.StatusMethodNotAllowed
	case errors.Is(err, dao.DatabaseIsReadonlyError):
		code = http.StatusMethodNotAllowed
	case errors.As(err, &fiberErr):
		code = fiberErr.Code
	}

	rootCause := utils.RootCause(err)
	tmfError := TmfError{
		Status:  strconv.Itoa(code),
		Code:    "MAAS-0600",
		Reason:  rootCause.Error(),
		Message: utils.Iff(rootCause != err, err.Error(), ""),
		Meta:    meta,
		Id:      uuid.NewString(),
		NSType:  "NC.TMFErrorResponse.v1.0",
	}

	log.ErrorC(ctx.UserContext(), "[%v][%v] %s", tmfError.Code, tmfError.Id, err.Error())
	return ctx.Status(code).JSON(tmfError)
}

func ExtractAndValidateClassifier(fiberCtx *fiber.Ctx, handler func(ctx context.Context, classifier *model.Classifier) error) error {
	ctx := fiberCtx.UserContext()
	classifier, err := model.NewClassifierFromReq(string(fiberCtx.Body()))
	if err != nil {
		return utils.LogError(log, ctx, "failed to create classifier from request body: %v: %w", err.Error(), msg.BadRequest)
	}
	err = v.ValidateClassifier(&classifier)
	if err != nil {
		return utils.LogError(log, ctx, "incorrect classifier: %w", err) // error already rooted from msg.BadRequest in ValidateClassifier
	}

	return handler(ctx, &classifier)
}

func WithYaml[T any](next func(*fiber.Ctx, *T) error) func(*fiber.Ctx) error {
	return WithBody(yaml.Unmarshal, next)
}

func WithJson[T any](next func(*fiber.Ctx, *T) error) func(*fiber.Ctx) error {
	return WithBody(json.Unmarshal, next)
}

func WithBody[T any](bodyParser func(data []byte, v any) error, next func(*fiber.Ctx, *T) error) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		var body T
		err := bodyParser(ctx.Body(), &body)
		if err != nil {
			return utils.LogError(log, ctx.UserContext(), "failed to parse body: %s: %w", err.Error(), msg.BadRequest)
		}
		if err := v.Get().Struct(body); err != nil {
			return err
		}
		return next(ctx, &body)
	}
}

func FallbackCrApiVersion(oldCrApi string) func(*fiber.Ctx) error {
	return func(ctx *fiber.Ctx) error {
		if oldCrApi != "" {
			body := ctx.Body()
			modifiedBody := bytes.ReplaceAll(body, []byte(oldCrApi), []byte("core.qubership.org/v1"))
			ctx.Request().SetBody(modifiedBody)
		}
		return ctx.Next()
	}
}
