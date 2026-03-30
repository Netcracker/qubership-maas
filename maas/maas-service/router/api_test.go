package router_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-maas/controller"
	controllerBluegreenV1 "github.com/netcracker/qubership-maas/controller/bluegreen/v1"
	controllerCompositeV1 "github.com/netcracker/qubership-maas/controller/composite/v1"
	controllerDeclarationsV1 "github.com/netcracker/qubership-maas/controller/declarations/v1"
	v1 "github.com/netcracker/qubership-maas/controller/v1"
	v2 "github.com/netcracker/qubership-maas/controller/v2"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/eventbus"
	"github.com/netcracker/qubership-maas/keymanagement"
	"github.com/netcracker/qubership-maas/monitoring"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/router"
	"github.com/netcracker/qubership-maas/service/auth"
	"github.com/netcracker/qubership-maas/service/bg2"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/bg_service"
	"github.com/netcracker/qubership-maas/service/cleanup"
	"github.com/netcracker/qubership-maas/service/composite"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	"github.com/netcracker/qubership-maas/service/cr"
	"github.com/netcracker/qubership-maas/service/instance"
	"github.com/netcracker/qubership-maas/service/kafka"
	"github.com/netcracker/qubership-maas/service/kafka/helper"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/service/tenant"
	"github.com/netcracker/qubership-maas/watchdog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTokenVerifier struct {
	token     string
	username  string
	namespace string
}

func (mv mockTokenVerifier) Verify(ctx context.Context, rawToken string) (*jwt.Token, error) {
	if rawToken != mv.token {
		return nil, errors.Join(errors.New("invalid token"), msg.AuthError)
	}
	return &jwt.Token{
		Claims: jwt.MapClaims{
			"kubernetes.io": map[string]any{
				"namespace":      mv.namespace,
				"serviceaccount": map[string]any{"name": mv.username},
			},
		},
	}, nil
}

func TestK8s_M2M_FeatureToggle(t *testing.T) {
	testNamespaceName := "test-namespace"
	validToken := "valid_token"
	tokenVerifier := mockTokenVerifier{
		token:     validToken,
		username:  "test-service",
		namespace: testNamespaceName,
	}

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		app := initApp(t, baseDao, tokenVerifier, true)

		config := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
`
		req := httptest.NewRequest(http.MethodPost, "/api/v2/config", strings.NewReader(config))
		req.Header.Set("Content-Type", "application/x-yaml")
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", validToken))
		resp, _ := app.Test(req)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

func initApp(t *testing.T, baseDao *dao.BaseDaoImpl, tokenVerifier mockTokenVerifier, k8sJwtEnabled bool) *fiber.App {
	ctx := t.Context()
	configloader.InitWithSourcesArray(configloader.BasePropertySources(configloader.YamlPropertySourceParams{ConfigFilePath: "../application.yaml"}))

	eventBus := eventbus.NewEventBus(eventbus.NewEventbusDao(baseDao))
	require.NoError(t, eventBus.Start(ctx))

	compositeRegistrationService := composite.NewRegistrationService(composite.NewPGRegistrationDao(baseDao))
	keyManagementHelper := keymanagement.NewPlain()
	bgService := bg_service.NewBgService(bg_service.NewBgServiceDao(baseDao))
	domainDao := domain.NewBGDomainDao(baseDao)
	bgDomainService := domain.NewBGDomainService(domainDao)
	authService := auth.NewAuthService(auth.NewAuthDao(baseDao), compositeRegistrationService, bgDomainService, tokenVerifier)

	kafkaHelper := helper.CreateKafkaHelper(ctx)
	kafkaInstanceService := instance.NewKafkaInstanceService(instance.NewKafkaInstancesDao(baseDao, domainDao), kafkaHelper)
	rabbitInstanceService := instance.NewRabbitInstanceService(instance.NewRabbitInstancesDao(baseDao))
	instanceWatchdog := watchdog.NewBrokerInstancesMonitor(kafkaInstanceService, rabbitInstanceService, time.Second*5)
	instanceWatchdog.Start(ctx)

	auditService := monitoring.NewAuditor(monitoring.NewDao(baseDao), dr.Disabled, instanceWatchdog.GetStatus)
	rabbitService := rabbit_service.NewProdMode(
		rabbit_service.NewRabbitService(rabbit_service.NewRabbitServiceDao(baseDao, bgDomainService.FindByNamespace), rabbitInstanceService, keyManagementHelper, auditService, bgService, bgDomainService, authService),
		false,
	)
	kafkaService := kafka.NewProdMode(
		kafka.NewKafkaService(kafka.NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace), kafkaInstanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService),
		false,
	)
	tenantService := tenant.NewTenantService(tenant.NewTenantServiceDaoImpl(baseDao), kafkaService)
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

	waitListDao := cr.NewPGWaitListDao(baseDao)
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
	healthAggregator := watchdog.NewHealthAggregator(baseDao.IsAvailable, instanceWatchdog.All)
	return router.CreateApi(ctx, controllers, healthAggregator, authService, k8sJwtEnabled)
}
