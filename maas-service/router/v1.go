package router

import (
	"github.com/gofiber/fiber/v2"
	"maas/maas-service/controller"
	"maas/maas-service/model"
)

// DEPRECATED
// Extracted into a separate function for ease of removal in the future
func createV1Api(app *fiber.App, controllers ApiControllers, roles func(roles ...model.RoleName) fiber.Handler) {
	apiV1 := app.Group("/api/v1")

	// general controller
	apiV1.Delete("/namespace", roles(model.ManagerRole), controllers.GeneralController.DeleteNamespace)

	// account controller
	apiV1.Post("/auth/account/client", roles(model.ManagerRole), controllers.AccountController.CreateAccount)

	apiV1.Delete("/auth/account/client", roles(model.ManagerRole), controllers.AccountController.DeleteClientAccount)

	apiV1.Get("/auth/accounts", roles(model.ManagerRole), controllers.AccountController.GetAllAccounts)

	apiV1.Post("/auth/account/manager", controllers.AccountController.SaveManagerAccount)

	// rabbit api
	apiV1.Group("/rabbit/instance*", roles(model.ManagerRole))
	apiV1.Get("/rabbit/instances", controllers.InstanceController.GetAllRabbitInstances)
	apiV1.Post("/rabbit/instance", controllers.InstanceController.RegisterNewRabbitInstance)
	apiV1.Put("/rabbit/instance", controllers.InstanceController.UpdateRabbitInstance)
	apiV1.Delete("/rabbit/instance", controllers.InstanceController.UnregisterRabbitInstance)
	apiV1.Put("/rabbit/instance/default", controllers.InstanceController.SetDefaultRabbitInstance)

	apiV1.Group("/rabbit/vhost", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Post("/rabbit/vhost/get-by-classifier", controllers.VHostController.GetVHostAndConfigByClassifier)
	apiV1.Post("/rabbit/vhost", controllers.VHostController.GetOrCreateVHost)
	apiV1.Delete("/rabbit/vhost", controllers.VHostController.DeleteVHost)

	// kafka api
	apiV1.Group("/kafka/instance*", roles(model.ManagerRole))
	apiV1.Get("/kafka/instances", controllers.InstanceController.GetAllKafkaInstances)
	apiV1.Post("/kafka/instance", controllers.InstanceController.RegisterNewKafkaInstance)
	apiV1.Put("/kafka/instance", controllers.InstanceController.UpdateKafkaInstance)
	apiV1.Delete("/kafka/instance", controllers.InstanceController.UnregisterKafkaInstance)
	apiV1.Put("/kafka/instance/default", controllers.InstanceController.SetDefaultKafkaInstance)

	apiV1.Group("/kafka/topic*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Post("/kafka/topic/get-by-classifier", controllers.TopicControllerV1.GetTopicByClassifier)
	apiV1.Post("/kafka/topic/search", controllers.TopicControllerV1.SearchTopics)
	apiV1.Post("/kafka/topic", controller.WithJson[model.TopicRegistrationReqDto](controllers.TopicControllerV1.GetOrCreateTopic))
	apiV1.Delete("/kafka/topic", controllers.TopicController.DeleteTopic)

	apiV1.Group("/kafka/topic-template*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Get("/kafka/topic-templates", controllers.TopicController.GetKafkaTopicTemplatesByNamespace)
	apiV1.Delete("/kafka/topic-template", controllers.TopicController.DeleteTemplate)

	apiV1.Group("/kafka/lazy-topic*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Post("/kafka/lazy-topic/definition", controllers.TopicController.DefineLazyTopic)
	apiV1.Post("/kafka/lazy-topic", controllers.TopicController.GetOrCreateLazyTopic)
	apiV1.Get("/kafka/lazy-topics/definitions", controllers.TopicController.GetLazyTopicsByNamespace)
	apiV1.Delete("/kafka/lazy-topic/definition", controllers.TopicController.DeleteLazyTopic)

	apiV1.Get("/kafka/tenant-topics", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TopicController.GetTenantTopicsByNamespace)

	apiV1.Group("/kafka/instance-designator*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Get("/kafka/instance-designator", controllers.InstanceController.GetKafkaInstanceDesignatorsByNamespace)
	apiV1.Delete("/kafka/instance-designator", controllers.InstanceController.DeleteKafkaInstanceDesignatorByNamespace)

	apiV1.Group("/rabbit/instance-designator*", roles(model.AgentRole), controller.RequiresNamespaceHeader)
	apiV1.Get("/rabbit/instance-designator", controllers.InstanceController.GetRabbitInstanceDesignatorsByNamespace)
	apiV1.Delete("/rabbit/instance-designator", controllers.InstanceController.DeleteRabbitInstanceDesignatorByNamespace)

	// common configuration endpoint
	apiV1.Post("/config", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.ConfiguratorController.ApplyConfig)

	// tenant api
	apiV1.Post("/synchronize-tenants", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TenantController.SyncTenants)
	apiV1.Get("/tenants", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.TenantController.GetTenantsByNamespace)

	// blue-green v1 api
	apiV1.Post("/bg-status", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.BgController.ApplyBgStatus)
	apiV1.Get("/bg-status", roles(model.AgentRole), controller.RequiresNamespaceHeader, controllers.BgController.GetBgStatusByNamespace)
}
