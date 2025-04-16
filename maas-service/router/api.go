package router

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/prometheus/client_golang/prometheus"
	"maas/maas-service/controller"
	bluegreenV1 "maas/maas-service/controller/bluegreen/v1"
	compositeV1 "maas/maas-service/controller/composite/v1"
	declarationsV1 "maas/maas-service/controller/declarations/v1"
	v1 "maas/maas-service/controller/v1"
	v2 "maas/maas-service/controller/v2"
	"maas/maas-service/docs"
	"maas/maas-service/model"
	"maas/maas-service/service/auth"
	"maas/maas-service/utils"
	"maas/maas-service/watchdog"
	"net/http"
	"time"
)

var log logging.Logger

// @title Maas Service API
// @description API for Maas Service.
// @Produce json
// @securityDefinitions.basic BasicAuth
// @scope.manager
// @scope.agent
// @in header
// @name Authorization
func init() {
	log = logging.GetLogger("controller")
}

type ApiControllers struct {
	AccountController               *controller.AccountController
	InstanceController              *controller.InstanceController
	VHostController                 *controller.VHostController
	ConfiguratorController          *controller.ConfiguratorController
	GeneralController               *controller.GeneralController
	TopicController                 *controller.TopicController
	TopicControllerV1               *v1.TopicController
	TopicControllerV2               *v2.TopicController
	TenantController                *controller.TenantController
	BgController                    *controller.BgController
	DiscrepancyController           *controller.DiscrepancyController
	Bg2Controller                   *bluegreenV1.Controller
	CustomResource                  *declarationsV1.CustomResourceController
	CompositeRegistrationController *compositeV1.RegistrationController
}

func CreateApi(ctx context.Context, controllers ApiControllers, healthService *watchdog.HealthAggregator, authService auth.AuthService) *fiber.App {
	log.InfoC(ctx, "Creating API controller")
	app := fiber.New(fiber.Config{
		IdleTimeout:  30 * time.Second,
		ErrorHandler: controller.TmfErrorHandler,
		WriteTimeout: 30 * time.Second,
		ReadTimeout:  130 * time.Second,
	})

	fallbackCrApiVersion := configloader.GetKoanf().String("fallback.cr.apiversion")
	log.InfoC(ctx, "Fallback CR API version: %s", fallbackCrApiVersion)

	// propagate root context to
	app.Use(propagateContext(ctx))
	// fix for stupid deployer polling to update port forward
	app.Use(indexPage(ctx))
	// prometheus metrics
	app.Use(prometheusHandler(app))
	// WA to avoid spam about a lot of /health requests in logs
	app.Use(healthShortcut(healthService.Status))
	app.Use(controller.ExtractOrAttachXRequestId)
	app.Use(controller.LogRequest)
	app.Use(controller.ExtractRequestContext)
	// swagger
	app.Get("/swagger-ui/swagger.json", func(ctx *fiber.Ctx) error {
		ctx.Set("Content-Type", "application/json")
		return ctx.Status(http.StatusOK).SendString(docs.SwaggerInfo.ReadDoc())
	})
	// ==============================================================================
	// Main API
	// ==============================================================================
	app.Get("/api-version", controllers.GeneralController.ApiVersion)

	apiV2 := app.Group("/api/v2")
	apiBGV1 := app.Group("/api/bluegreen/v1/")
	apiCRV1 := app.Group("/api/declarations/v1/")
	apiCompositeV1 := app.Group("/api/composite/v1/")

	roles := func(roles ...model.RoleName) fiber.Handler {
		return controller.SecurityMiddleware(roles, authService.IsAccessGranted)
	}

	createV1Api(app, controllers, roles)

	// general api
	apiV2.Delete("/namespace", roles(model.ManagerRole), controllers.GeneralController.DeleteNamespace)
	// TODO why ParseRequestParametersFromConfig not a part of controllers.ConfiguratorController.ApplyConfigV2 ?
	apiV2.Post("/config", roles(model.AgentRole), controller.ParseRequestParametersFromConfig, controllers.ConfiguratorController.ApplyConfigV2)

	// account api
	apiV2.Post("/auth/account/client", roles(model.ManagerRole), controllers.AccountController.CreateAccount)
	apiV2.Delete("/auth/account/client", roles(model.ManagerRole), controllers.AccountController.DeleteClientAccount)
	apiV2.Get("/auth/accounts", roles(model.ManagerRole), controllers.AccountController.GetAllAccounts)
	apiV2.Post("/auth/account/manager", controllers.AccountController.SaveManagerAccount)
	apiV2.Put("/auth/account/manager/:name/password", roles(model.ManagerRole), controllers.AccountController.UpdatePassword)

	// rabbit api
	apiV2.Group("/rabbit/instance*", roles(model.ManagerRole))
	apiV2.Get("/rabbit/instances", controllers.InstanceController.GetAllRabbitInstances)
	apiV2.Post("/rabbit/instance", controllers.InstanceController.RegisterNewRabbitInstance)
	apiV2.Put("/rabbit/instance", controllers.InstanceController.UpdateRabbitInstance)
	apiV2.Delete("/rabbit/instance", controllers.InstanceController.UnregisterRabbitInstance)
	apiV2.Put("/rabbit/instance/default", controllers.InstanceController.SetDefaultRabbitInstance)

	apiV2.Group("/rabbit/vhost", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Post("/rabbit/vhost/get-by-classifier", controllers.VHostController.GetVHostAndConfigByClassifier)
	apiV2.Post("/rabbit/vhost", controllers.VHostController.GetOrCreateVHost)
	apiV2.Post("/rabbit/vhost/search", controllers.VHostController.SearchVhosts)
	apiV2.Delete("/rabbit/vhost", controllers.VHostController.DeleteVHost)

	apiV2.Group("/rabbit/instance-designator*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Get("/rabbit/instance-designator", controllers.InstanceController.GetRabbitInstanceDesignatorsByNamespace)
	apiV2.Delete("/rabbit/instance-designator", controllers.InstanceController.DeleteRabbitInstanceDesignatorByNamespace)

	apiV2.Group("/rabbit", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Get("/rabbit/validations", controllers.VHostController.ValidateRabbitConfigs)
	apiV2.Post("/rabbit/recovery/:namespace", controllers.VHostController.RecoverNamespace)
	apiV2.Post("/rabbit/password-rotation", controllers.VHostController.RotatePasswords)

	// kafka api
	apiV2.Group("/kafka/instance*", roles(model.ManagerRole))
	apiV2.Get("/kafka/instances", controllers.InstanceController.GetAllKafkaInstances)
	apiV2.Post("/kafka/instance", controllers.InstanceController.RegisterNewKafkaInstance)
	apiV2.Put("/kafka/instance", controllers.InstanceController.UpdateKafkaInstance)
	apiV2.Delete("/kafka/instance", controllers.InstanceController.UnregisterKafkaInstance)
	apiV2.Put("/kafka/instance/default", controllers.InstanceController.SetDefaultKafkaInstance)

	apiV2.Group("/kafka/topic-template*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Get("/kafka/topic-templates", controllers.TopicController.GetKafkaTopicTemplatesByNamespace)
	apiV2.Delete("/kafka/topic-template", controllers.TopicController.DeleteTemplate)

	apiV2.Group("/kafka/lazy-topic*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Post("/kafka/lazy-topic/definition", controllers.TopicController.DefineLazyTopic)
	apiV2.Post("/kafka/lazy-topic", controllers.TopicController.GetOrCreateLazyTopic)
	apiV2.Get("/kafka/lazy-topics/definitions", controllers.TopicController.GetLazyTopicsByNamespace)
	apiV2.Delete("/kafka/lazy-topic/definition", controllers.TopicController.DeleteLazyTopic)

	apiV2.Get("/kafka/tenant-topics", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TopicController.GetTenantTopicsByNamespace)
	apiV2.Delete("/kafka/tenant-topic", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TopicController.DeleteTenantTopic)

	apiV2.Group("/kafka/instance-designator*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Get("/kafka/instance-designator", controllers.InstanceController.GetKafkaInstanceDesignatorsByNamespace)
	apiV2.Delete("/kafka/instance-designator", controllers.InstanceController.DeleteKafkaInstanceDesignatorByNamespace)

	apiV2.Group("/kafka/topic*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV2.Delete("/kafka/topic", controllers.TopicController.DeleteTopic)
	apiV2.Post("/kafka/topic/watch-create", controller.RequiresNamespaceHeader, controllers.TopicController.WatchTopicsCreate)
	apiV2.Post("/kafka/topic", controller.WithJson[model.TopicRegistrationReqDto](controllers.TopicControllerV2.GetOrCreateTopic))
	apiV2.Post("/kafka/topic/search", controllers.TopicControllerV2.SearchTopics)
	apiV2.Post("/kafka/topic/get-by-classifier", controllers.TopicControllerV2.GetTopicByClassifier)
	//todo remove in future releases
	apiV2.Post("/kafka/topic/sync/:namespace", controllers.TopicControllerV2.SyncAllTopicsToKafka)

	apiV2.Post("/kafka/recovery/:namespace", controllers.TopicControllerV2.SyncAllTopicsToKafka)
	apiV2.Post("/kafka/recover-topic", controllers.TopicControllerV2.SyncTopicToKafka)

	apiV2.Get("/kafka/discrepancy-report/:namespace", controllers.DiscrepancyController.GetReport)

	// tenant api
	apiV2.Post("/synchronize-tenants", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TenantController.SyncTenants)
	apiV2.Get("/tenants", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TenantController.GetTenantsByNamespace)

	// blue-green v1 api
	apiV2.Post("/bg-status", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.BgController.ApplyBgStatus)
	apiV2.Get("/bg-status", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.BgController.GetBgStatusByNamespace)

	// monitoring api
	apiV2.Get("/monitoring/entity-distribution", roles(model.ManagerRole), controllers.GeneralController.GetMonitoringEntities)
	apiV2.Get("/monitoring/entity-request-audit", controllers.GeneralController.GetMonitoringEntityRequests)

	// blue-green v2 api
	apiBGV1.Get("/operation/list-domains", roles(model.AnonymousRole, model.BgOperatorRole), controllers.Bg2Controller.ListDomains)
	apiBGV1.Post("/operation/init-domain", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.InitDomain))
	apiBGV1.Post("/operation/warmup", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.Warmup))
	apiBGV1.Post("/operation/promote", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.Promote))
	apiBGV1.Post("/operation/rollback", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.Rollback))
	apiBGV1.Post("/operation/commit", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.Commit))
	apiBGV1.Delete("/operation/destroy-domain", roles(model.AnonymousRole, model.BgOperatorRole), controller.WithJson(controllers.Bg2Controller.DestroyDomain))

	// custom resources api
	apiCRV1.Post("/apply", roles(model.AgentRole), controller.FallbackCrApiVersion(fallbackCrApiVersion), controller.WithYaml(controllers.CustomResource.Create))
	apiCRV1.Delete("/apply", roles(model.AgentRole), controller.WithYaml(controllers.CustomResource.Delete))
	apiCRV1.Get("/operation/:trackingId/status", roles(model.AgentRole), controllers.CustomResource.Status)
	apiCRV1.Post("/operation/:trackingId/terminate", roles(model.AgentRole), controllers.CustomResource.Terminate)

	// composite api
	apiCompositeV1.Get("/structure/:id", roles(model.AgentRole), controllers.CompositeRegistrationController.GetById) // deprecated
	apiCompositeV1.Get("/structures/:id", roles(model.AgentRole), controllers.CompositeRegistrationController.GetById)
	apiCompositeV1.Get("/structures/", roles(model.AgentRole), controllers.CompositeRegistrationController.GetAll)
	apiCompositeV1.Delete("/structure/:id", roles(model.AgentRole), controllers.CompositeRegistrationController.DeleteById) // deprecated
	apiCompositeV1.Delete("/structures/:id/delete", roles(model.AgentRole), controllers.CompositeRegistrationController.DeleteById)
	apiCompositeV1.Post("/structure", roles(model.AgentRole), controller.WithJson[compositeV1.RegistrationRequest](controllers.CompositeRegistrationController.Create)) // deprecated
	apiCompositeV1.Post("/structures", roles(model.AgentRole), controller.WithJson[compositeV1.RegistrationRequest](controllers.CompositeRegistrationController.Create))

	return app
}

func propagateContext(ctx context.Context) func(fiberCtx *fiber.Ctx) error {
	return func(fiberCtx *fiber.Ctx) error {
		fiberCtx.SetUserContext(ctx)
		return fiberCtx.Next()
	}
}

func healthShortcut(health func() watchdog.AggregatedStatus) func(*fiber.Ctx) error {
	return func(fiberCtx *fiber.Ctx) error {
		if fiberCtx.OriginalURL() == "/health" {
			return fiberCtx.Status(http.StatusOK).JSON(health())
		} else {
			return fiberCtx.Next()
		}
	}
}

func indexPage(_ context.Context) func(fiberCtx *fiber.Ctx) error {
	return func(fiberCtx *fiber.Ctx) error {
		if fiberCtx.OriginalURL() == "/" {
			_, _ = fiberCtx.WriteString("Nothing to see here")
			return nil
		} else {
			return fiberCtx.Next()
		}
	}
}

func prometheusHandler(app fiber.Router) func(ctx *fiber.Ctx) error {
	promHandler := utils.NewWithRegistry(prometheus.DefaultRegisterer, "maas-service", "http", "", nil)
	promHandler.RegisterAt(app, "/prometheus")
	return promHandler.Middleware
}
