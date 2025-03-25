package main

import (
	"context"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/pprof"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/baseproviders"
	"github.com/netcracker/qubership-core-lib-go/v3/context-propagation/ctxmanager"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/rcrowley/go-metrics"
	"maas/maas-service/controller"
	controllerBluegreenV1 "maas/maas-service/controller/bluegreen/v1"
	controllerCompositeV1 "maas/maas-service/controller/composite/v1"
	controllerDeclarationsV1 "maas/maas-service/controller/declarations/v1"
	v1 "maas/maas-service/controller/v1"
	v2 "maas/maas-service/controller/v2"
	"maas/maas-service/dao"
	"maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/eventbus"
	"maas/maas-service/keymanagement"
	"maas/maas-service/monitoring"
	"maas/maas-service/postdeploy"
	"maas/maas-service/router"
	"maas/maas-service/service/auth"
	"maas/maas-service/service/bg2"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/bg_service"
	"maas/maas-service/service/cleanup"
	"maas/maas-service/service/composite"
	"maas/maas-service/service/configurator_service"
	"maas/maas-service/service/cr"
	"maas/maas-service/service/instance"
	"maas/maas-service/service/kafka"
	"maas/maas-service/service/kafka/helper"
	"maas/maas-service/service/rabbit_service"
	"maas/maas-service/service/tenant"
	"maas/maas-service/utils"
	"maas/maas-service/watchdog"
	"os"
	"sync/atomic"
	"time"
	// swagger docs
	_ "maas/maas-service/docs"
)

var (
	log               logging.Logger
	ctx, globalCancel = context.WithCancel(
		context.WithValue(
			context.Background(), "requestId", "",
		),
	)
	exitCode = atomic.Int32{}
)

func init() {
	configloader.InitWithSourcesArray(configloader.BasePropertySources())
	ctxmanager.Register(baseproviders.Get())
	log = logging.GetLogger("server")
}

//go:generate go run github.com/swaggo/swag/cmd/swag@v1.8.1 init --generalInfo /router/api.go --parseDependency --parseDepth 2
func main() {
	// sarama memory leak PSUPCLFRM-3634
	metrics.UseNilMetrics = true

	startPProf()

	healthCheckInterval := configloader.GetKoanf().Duration("health.check.interval")

	drMode := dr.ModeFromString(configloader.GetKoanf().String("execution.mode"))
	isProdMode := configloader.GetKoanf().Bool("production.mode")
	log.InfoC(ctx, "Initializing service with: execution.mode=%v, production.mode=%v", drMode, isProdMode)

	pg := createDao(ctx, drMode, healthCheckInterval)

	eventBus := eventbus.NewEventBus(eventbus.NewEventbusDao(pg))
	if err := eventBus.Start(ctx); err != nil {
		log.PanicC(ctx, "EventBus start failed: %v", err)
	}

	compositeRegistrationService := composite.NewRegistrationService(composite.NewPGRegistrationDao(pg))
	keyManagementHelper := keymanagement.NewPlain()
	bgService := bg_service.NewBgService(bg_service.NewBgServiceDao(pg))
	domainDao := domain.NewBGDomainDao(pg)
	bgDomainService := domain.NewBGDomainService(domainDao)
	authService := auth.NewAuthService(auth.NewAuthDao(pg), compositeRegistrationService, bgDomainService)

	kafkaHelper := helper.CreateKafkaHelper(ctx)
	kafkaInstanceService := instance.NewKafkaInstanceService(instance.NewKafkaInstancesDao(pg, domainDao), kafkaHelper)
	rabbitInstanceService := instance.NewRabbitInstanceService(instance.NewRabbitInstancesDao(pg))
	instanceWatchdog := watchdog.NewBrokerInstancesMonitor(kafkaInstanceService, rabbitInstanceService, healthCheckInterval)
	instanceWatchdog.Start(ctx)

	auditService := monitoring.NewAuditor(monitoring.NewDao(pg), drMode, instanceWatchdog.GetStatus)
	rabbitService := rabbit_service.NewProdMode(
		rabbit_service.NewRabbitService(rabbit_service.NewRabbitServiceDao(pg, bgDomainService.FindByNamespace), rabbitInstanceService, keyManagementHelper, auditService, bgService, bgDomainService, authService),
		isProdMode,
	)
	kafkaService := kafka.NewProdMode(
		kafka.NewKafkaService(kafka.NewKafkaServiceDao(pg, bgDomainService.FindByNamespace), kafkaInstanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService),
		isProdMode,
	)
	tenantService := tenant.NewTenantService(tenant.NewTenantServiceDaoImpl(pg), kafkaService)
	configService := configurator_service.NewConfiguratorService(
		kafkaInstanceService,
		rabbitInstanceService,
		rabbitService,
		tenantService,
		kafkaService,
		bgService,
		bgDomainService,
		compositeRegistrationService,
	)
	bgManagerService := bg2.NewManager(bgDomainService, kafkaService, rabbitService)

	waitListDao := cr.NewPGWaitListDao(pg)
	customResourceProcessorService := cr.NewCustomResourceProcessorService(waitListDao, configService, kafkaService, rabbitService)

	cleanupService := cleanup.NewNamespaceCleanupService(
		kafkaService.CleanupNamespace,
		rabbitService.CleanupNamespace,
		tenantService.DeleteTenantsByNamespace,
		auditService.CleanupData,
		bgService.CleanupNamespace,
		compositeRegistrationService.CleanupNamespace,
		kafkaInstanceService.DeleteKafkaInstanceDesignatorByNamespace,
		rabbitInstanceService.DeleteRabbitInstanceDesignatorByNamespace,
		customResourceProcessorService.CleanupNamespace,
	)

	controllers := router.ApiControllers{
		AccountController:               controller.NewAccountController(authService),
		InstanceController:              controller.NewInstanceController(kafkaInstanceService, rabbitInstanceService, authService),
		VHostController:                 controller.NewVHostController(rabbitService, authService),
		ConfiguratorController:          controller.NewConfiguratorController(configService, authService),
		GeneralController:               controller.NewGeneralController(cleanupService, authService, auditService),
		TopicController:                 controller.NewTopicController(kafkaService, authService),
		TopicControllerV1:               v1.NewTopicController(kafkaService, authService),
		TopicControllerV2:               v2.NewTopicController(kafkaService, authService),
		TenantController:                controller.NewTenantController(tenantService, authService),
		BgController:                    controller.NewBgController(bgService, authService),
		Bg2Controller:                   controllerBluegreenV1.NewController(bgManagerService),
		DiscrepancyController:           controller.NewDiscrepancyController(kafkaService),
		CustomResource:                  controllerDeclarationsV1.NewCustomResourceController(customResourceProcessorService),
		CompositeRegistrationController: controllerCompositeV1.NewRegistrationController(compositeRegistrationService),
	}

	if drMode.IsActive() {
		err := kafkaService.MigrateKafka(ctx)
		if err != nil {
			log.PanicC(ctx, err.Error())
		}
	}

	healthAggregator := watchdog.NewHealthAggregator(pg.IsAvailable, instanceWatchdog.All)
	app := router.CreateApi(ctx, controllers, healthAggregator, authService)

	utils.RegisterShutdownHook(func(code int) {
		// save exit code to be used in Exit() call
		exitCode.Store(int32(code))

		// start shutdown
		if err := app.Shutdown(); err != nil {
			log.ErrorC(ctx, "MaaS error during server shutdown: %v", err)
		}
		globalCancel()
		pg.Close()
	})

	if drMode.IsActive() {
		log.InfoC(ctx, "Run postdeploy actions...")
		err := postdeploy.RunPostdeployScripts(ctx, authService, rabbitInstanceService, kafkaInstanceService)
		if err != nil {
			log.PanicC(ctx, err.Error())
		}
		log.InfoC(ctx, "Postdeploy actions finished")
	} else {
		log.InfoC(ctx, "Skip postdeploy actions due to dr mode in on")
	}

	serverBind := configloader.GetOrDefaultString("http.server.bind", ":8080")
	log.InfoC(ctx, "Starting server on %v", serverBind)
	if err := app.Listen(serverBind); err != nil {
		log.PanicC(ctx, err.Error())
	}

	log.InfoC(ctx, "Server gracefully finished with exit code: %d", exitCode.Load())
	os.Exit(int(exitCode.Load()))
}

func startPProf() {
	go func() {
		app := fiber.New(fiber.Config{DisableStartupMessage: true})
		app.Use(pprof.New())
		log.Debugf("run pprof on 127.0.0.1:6060")
		app.Listen("127.0.0.1:6060")
	}()
}

func createDao(ctx context.Context, drMode dr.Mode, healthCheckInterval time.Duration) dao.BaseDao {
	configSection := configloader.GetKoanf()
	dbConfig := db.Config{
		Addr:          configSection.String("db.postgresql.address"),
		User:          configSection.String("db.postgresql.username"),
		Password:      configSection.String("db.postgresql.password"),
		Database:      configSection.String("db.postgresql.database"),
		PoolSize:      configSection.Int("db.pool.size"),
		ConnectionTtl: configSection.Duration("db.connection.ttl"),
		DrMode:        drMode,
		TlsEnabled:    configSection.Bool("db.postgresql.tls.enabled"),
		TlsSkipVerify: configSection.Bool("db.postgresql.tls.skipverify"),
		CipherKey:     configSection.MustString("db.cipher.key"),
	}

	log.Info("Initialize dao with db: %+v", dbConfig)
	daoInstance := dao.New(&dbConfig)

	if err := daoInstance.StartMonitor(ctx, healthCheckInterval); err != nil {
		log.PanicC(ctx, "error start db monitor: %v", err)
	}

	go func() {
		// give CPU a time for other tasks that have much higher priority that this shitty cache
		if !utils.CancelableSleep(ctx, 30*time.Second) {
			return
		}

		cacheEnabled := configSection.Bool("db.cache.enabled")
		if cacheEnabled && drMode.IsActive() {
			err := daoInstance.StartCache(ctx,
				configSection.Duration("db.cache.fullResyncInterval"),
			)
			if err != nil {
				log.ErrorC(ctx, "Error start cache: %s", err)
			}
		} else {
			log.InfoC(ctx, "Cache is in inactive state. Config cache enabled: '%t', DR mode: '%s'", cacheEnabled, drMode)
		}
	}()

	return daoInstance
}
