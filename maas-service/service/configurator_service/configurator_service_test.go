package configurator_service

import (
	"context"
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/golang/mock/gomock"
	_ "github.com/proullon/ramsql/driver"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dao"
	dbc "maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/model"
	"maas/maas-service/monitoring"
	mock_auth "maas/maas-service/service/auth/mock"
	"maas/maas-service/service/bg2/domain"
	mock_domain "maas/maas-service/service/bg2/domain/mock"
	"maas/maas-service/service/bg_service"
	mock_configurator_service "maas/maas-service/service/configurator_service/mock"
	"maas/maas-service/service/instance"
	mock_instance "maas/maas-service/service/instance/mock"
	"maas/maas-service/service/kafka"
	mock_kafka_helper "maas/maas-service/service/kafka/helper/mock"
	"maas/maas-service/service/rabbit_service"
	"maas/maas-service/service/rabbit_service/helper"
	mock_rabbit_helper "maas/maas-service/service/rabbit_service/helper/mock"
	mock_rabbit_service "maas/maas-service/service/rabbit_service/mock"
	"maas/maas-service/service/tenant"
	"maas/maas-service/testharness"
	"strings"
	"testing"
	"time"
)
import (
	_assert "github.com/stretchr/testify/assert"
)

var (
	assertion                 *assert.Assertions
	mockCtrl                  *gomock.Controller
	ctx                       context.Context = nil
	rabbitHelper              *mock_rabbit_helper.MockRabbitHelper
	km                        *mock_rabbit_service.MockKeyManager
	auditService              *monitoring.MockAuditor
	authServiceMock           *mock_auth.MockAuthService
	kafkaServiceMock          *kafka.MockKafkaService
	tenantServiceMock         *mock_configurator_service.MockTenantService
	rabbitServiceMock         *mock_rabbit_service.MockRabbitService
	kafkaInstanceServiceMock  *mock_instance.MockKafkaInstanceService
	rabbitInstanceServiceMock *mock_instance.MockRabbitInstanceService

	compositeNamespaceManager *mock_configurator_service.MockRegistrationService

	rabbitHelperMockFunc rabbit_service.RabbitHelperFunc

	namespace         = "test-namespace"
	db                *pg.DB
	daoImpl           dao.BaseDao
	rabbitDaoImpl     rabbit_service.RabbitServiceDao
	bgDaoImpl         bg_service.BgServiceDao
	domainServiceImpl domain.BGDomainService
)

func testInitializer(t *testing.T) {
	ctx = context.Background()

	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	rabbitHelper = mock_rabbit_helper.NewMockRabbitHelper(mockCtrl)
	km = mock_rabbit_service.NewMockKeyManager(mockCtrl)
	auditService = monitoring.NewMockAuditor(mockCtrl)

	authServiceMock = mock_auth.NewMockAuthService(mockCtrl)
	kafkaServiceMock = kafka.NewMockKafkaService(mockCtrl)
	rabbitServiceMock = mock_rabbit_service.NewMockRabbitService(mockCtrl)
	kafkaInstanceServiceMock = mock_instance.NewMockKafkaInstanceService(mockCtrl)
	rabbitInstanceServiceMock = mock_instance.NewMockRabbitInstanceService(mockCtrl)
	tenantServiceMock = mock_configurator_service.NewMockTenantService(mockCtrl)
	compositeNamespaceManager = mock_configurator_service.NewMockRegistrationService(mockCtrl)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(ctx, requestContext)

	rabbitHelperMockFunc = func(ctx context.Context, s rabbit_service.RabbitService, instance *model.RabbitInstance, vhost *model.VHostRegistration, classifier *model.Classifier) (helper.RabbitHelper, error) {
		return rabbitHelper, nil
	}
}

func TestIncorrectInput(t *testing.T) {
	assert := _assert.New(t)

	cfg := `
---
Abc: cde
`

	service := NewConfiguratorService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := service.ApplyConfig(context.Background(), cfg, "ns")
	assert.Error(err)
}

func TestIncorrectKind(t *testing.T) {
	testInitializer(t)
	assert := _assert.New(t)

	cfg := `
---
apiVersion: cde
kind: foo
`

	service := NewConfiguratorService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := service.ApplyConfig(context.Background(), cfg, "ns")

	fmt.Printf("Results: %+v\n", err)

	assert.Error(err)
}

func TestKafkaUnknownField(t *testing.T) {
	testInitializer(t)
	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: test-namespace
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             wrongField: wrong
             classifier: { name: abc }
`

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(nil, nil, nil, nil, nil, nil, nil, nil)
	_, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal("Error during applying aggregated config, error: 'configurator_service: bad input, check correctness of your YAML', message: (can't map data in config to '{Kind:{ApiVersion: Kind:} Pragma:<nil> Spec:{Name: Classifier:{} ExternallyManaged:false Instance: NumPartitions:<nil> MinNumPartitions:<nil> ReplicationFactor:<nil> ReplicaAssignment:map[] Configs:map[] Template: Versioned:false}}', for config 'map[apiVersion:nc.maas.kafka/v1 kind:topic spec:map[classifier:map[name:abc] wrongField:wrong]]'. error: 'json: unknown field \"wrongField\"')", err.Error())
}

func TestConfigV2KafkaApply(t *testing.T) {
	testInitializer(t)
	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
 version: v1
 namespace: test-namespace
 services:
   - serviceName: order-processor
     config: |+
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: abc, namespace: test-ns }

         ---
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: cde, namespace: test-ns }

   - serviceName: order-executor
     config: |+
         ---
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: foo, namespace: test-ns }

         ---
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: bar, namespace: test-ns }
`

	mockCtrl := gomock.NewController(t)
	kafkaMock := kafka.NewMockKafkaService(mockCtrl)

	exp := &model.TopicRegistrationRespDto{}
	kafkaMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	gomock.InOrder(
		kafkaMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
		kafkaMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
		kafkaMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
		kafkaMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(nil, nil, nil, nil, kafkaMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.NoError(err)
	assert.Equal(4, len(result))

	assert.Equal(model.STATUS_OK, result[0].Result.Status)
}

func TestKafkaApply(t *testing.T) {
	testInitializer(t)
	assert := _assert.New(t)

	cfg := `
---
apiVersion: nc.maas.kafka/v1
kind: topic
spec:
 classifier:
   name: my_topic
   namespace: test-ns
`

	mockCtrl := gomock.NewController(t)
	kafkaMock := kafka.NewMockKafkaService(mockCtrl)

	exp := &model.TopicRegistrationRespDto{}
	kafkaMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	kafkaMock.EXPECT().
		GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, exp, nil).
		Times(1)

	service := NewConfiguratorService(nil, nil, nil, nil, kafkaMock, nil, nil, nil)
	results, err := service.ApplyConfig(context.Background(), cfg, "ns")

	assert.NoError(err)
	assert.Equal(1, len(results))
	assert.Equal(model.ConfigMsResult{Status: "ok", Error: "", Data: exp}, results[0].Result)
}

func TestConfigApplyError(t *testing.T) {
	testInitializer(t)
	assert := _assert.New(t)

	cfg := `
---
apiVersion: nc.maas.kafka/v1
kind: topic
spec:
 classifier:
   name: my_topic
   namespace: test-ns
`

	mockCtrl := gomock.NewController(t)
	kafkaMock := kafka.NewMockKafkaService(mockCtrl)

	kafkaMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	kafkaMock.EXPECT().
		GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil, errors.New("Err")).
		Times(1)
	service := NewConfiguratorService(nil, nil, nil, nil, kafkaMock, nil, nil, nil)
	results, err := service.ApplyConfig(context.Background(), cfg, "ns")

	assert.Error(err)
	assert.Equal(1, len(results))
	assert.Equal(model.ConfigMsResult{Status: "error", Error: "Error apply config: Err, \n\tConfig: &{Kind:{ApiVersion:nc.maas.kafka/v1 Kind:topic} Pragma:<nil> Spec:{Name: Classifier:{\"name\":\"my_topic\",\"namespace\":\"test-ns\"} ExternallyManaged:false Instance: NumPartitions:<nil> MinNumPartitions:<nil> ReplicationFactor:<nil> ReplicaAssignment:map[] Configs:map[] Template: Versioned:false}}", Data: nil},
		results[0].Result)
}

func TestConfigDesignatorWithoutDefaultInstance(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: namespace
  shared: |
    apiVersion: nc.maas.kafka/v2
    kind: instance-designator
    spec:
        namespace: namespace
        defaultInstance:
        selectors:
        - classifierMatch:
            name: orders
          instance: cpq-kafka-maas-test
    ---
    apiVersion: nc.maas.rabbit/v2
    kind: instance-designator
    spec:
        namespace: namespace
        defaultInstance: 
        selectors:
        - classifierMatch:
            name: orders
          instance: cpq-rabbit-maas-test
    ---
    apiVersion: nc.maas.kafka/v2
    kind: instance-designator
    spec:
        namespace: namespace1
        selectors:
        - classifierMatch:
            name: orders
          instance: cpq-kafka-maas-test
    ---
    apiVersion: nc.maas.rabbit/v2
    kind: instance-designator
    spec:
        namespace: namespace1
        selectors:
        - classifierMatch:
            name: orders
          instance: cpq-rabbit-maas-test
`

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(base)
		kafkaInstanceService := instance.NewKafkaInstanceServiceWithHealthChecker(
			instance.NewKafkaInstancesDao(base, domainDao),
			nil,
			func(kafkaInstance *model.KafkaInstance) error {
				return nil
			},
		)
		_, err := kafkaInstanceService.Register(ctxMock, &model.KafkaInstance{
			Id:           "cpq-kafka-maas-test",
			Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"test-kafka:9092"}},
			MaasProtocol: model.Plaintext,
		})
		assert.NoError(err)

		rabbitInstanceService := instance.NewRabbitInstanceServiceWithHealthChecker(
			instance.NewRabbitInstancesDao(base),
			func(rabbitInstance *model.RabbitInstance) error {
				return nil
			},
		)
		_, err = rabbitInstanceService.Register(ctxMock, &model.RabbitInstance{
			Id:       "cpq-rabbit-maas-test",
			ApiUrl:   "url",
			AmqpUrl:  "url",
			User:     "user",
			Password: "password",
			Default:  true,
		})
		assert.NoError(err)

		service := NewConfiguratorService(kafkaInstanceService, rabbitInstanceService, nil, nil, nil, nil, nil, nil)
		result, err := service.ApplyConfigV2(ctxMock, cfg)

		assert.NoError(err)
		assert.Equal(4, len(result))

		assert.Equal(model.STATUS_OK, result[0].Result.Status)
	})
}

func TestSharedConfigDesignatorAndKafkaTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
    version: v1
    namespace: namespace
    shared: |
      apiVersion: nc.maas.kafka/v2
      kind: instance-designator
      spec:
          namespace: namespace
          defaultInstance: cpq-kafka-maas-test
          selectors:
          - classifierMatch:
              name: orders
              namespace: namespace
            instance: cpq-kafka-maas-test
          - classifierMatch:
              name: orders-2
              namespace: namespace
            instance: cpq-kafka-maas-test
      ---
      apiVersion: nc.maas.kafka/v1
      kind: topic
      spec:
        classifier: { name: abc, namespace: namespace }
    services:
      - serviceName: order-processor
        config: |+
            apiVersion: nc.maas.kafka/v1
            kind: topic
            spec:
               classifier: { name: cde, namespace: namespace }
`

	exp := &model.TopicRegistrationRespDto{}
	kafkaServiceMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	gomock.InOrder(
		kafkaInstanceServiceMock.EXPECT().
			UpsertKafkaInstanceDesignator(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.NoError(err)
	assert.Equal(3, len(result))

	assert.Equal(model.STATUS_OK, result[0].Result.Status)
}

func TestSharedConfigParsingErr(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: namespace
  shared: |
    apiVersion: nc.maas.kafka/v2
    kind: instance-designator
    spec:
        namespace: namespace
        defaultInstance: cpq-kafka-maas-test
        selectors:
        - classifierMatch:
            name: orders
            namespace: namespace
            instance: cpq-kafka-maas-test
        - classifierMatch:
            name: orders-2
            namespace: namespace
            instance: cpq-kafka-maas-test
    ---
    apiVersion: nc.maas.kafka/v777
    kind: topic
    spec:
      classifier: { name: abc, namespace: namespace }
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: cde, namespace: namespace }
`

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal(0, len(result))
	assert.Equal("Error during parseInnerConfigsOfMs for shared config while in ApplyConfigV2: Error during applying aggregated config, error: 'configurator_service: bad input, check correctness of your YAML', message: (Unsupported object kind '{ApiVersion:nc.maas.kafka/v777 Kind:topic}' in inner config 'map[apiVersion:nc.maas.kafka/v777 kind:topic spec:map[classifier:map[name:abc namespace:namespace]]]')", err.Error())
}

func TestSharedConfigUnknownFieldErr(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: namespace
  wrong-name-for-shared: |
    apiVersion: nc.maas.kafka/v1
    kind: topic
    spec:
      classifier: { name: abc, namespace: namespace }
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: cde, namespace: namespace }
`

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal(0, len(result))
	assert.Equal("Error during applying aggregated config, error: 'configurator_service: bad input, check correctness of your YAML', message: (can't map data in aggregated config to: {Kind:{ApiVersion: Kind:} Pragma:<nil> Spec:{Version: Namespace: BaseNamespace: SharedConfigs:<nil> ServiceConfigs:[]}}, for init yaml: 'map[apiVersion:nc.maas.config/v2 kind:config spec:map[namespace:namespace services:[map[config:apiVersion: nc.maas.kafka/v1\nkind: topic\nspec:\n   classifier: { name: cde, namespace: namespace }\n serviceName:order-processor]] version:v1 wrong-name-for-shared:apiVersion: nc.maas.kafka/v1\nkind: topic\nspec:\n  classifier: { name: abc, namespace: namespace }\n]]'. error: json: unknown field \"wrong-name-for-shared\")", err.Error())
}

func TestSharedConfigErr(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: namespace
  shared: |
    apiVersion: nc.maas.kafka/v2
    kind: instance-designator
    spec:
        namespace: namespace
        defaultInstance: cpq-kafka-maas-test
        selectors:
        - classifierMatch:
            name: orders
            namespace: namespace
          instance: cpq-kafka-maas-test
        - classifierMatch:
            name: orders-2
            namespace: namespace
          instance: cpq-kafka-maas-test
    ---
    apiVersion: nc.maas.kafka/v1
    kind: topic
    spec:
      classifier: { name: abc, namespace: namespace }
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v1
          kind: topic
          spec:
             classifier: { name: cde, namespace: namespace }
`
	kafkaServiceMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	gomock.InOrder(
		kafkaInstanceServiceMock.EXPECT().
			UpsertKafkaInstanceDesignator(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(errors.New("some dao error")).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, &model.TopicRegistrationRespDto{}, nil).
			AnyTimes(),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal(3, len(result))
	assert.Equal("Error during applying aggregated config, error: 'configurator_service: server error for internal config of microservice', message: (Error during applying inner config for microservice with name '', err: some dao error)", err.Error())
}

func TestSharedConfigDesignatorAndKafkaTopicError(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
 version: v1
 namespace: namespace
 shared: |
   apiVersion: nc.maas.kafka/v2
   kind: instance-designator
   spec:
       namespace: namespace
       defaultInstance: cpq-kafka-maas-test
       selectors:
       - classifierMatch:
           name: orders
           namespace: namespace
         instance: cpq-kafka-maas-test
       - classifierMatch:
           name: orders-2
           namespace: namespace
         instance: cpq-kafka-maas-test
   ---
   apiVersion: nc.maas.kafka/v1
   kind: topic
   spec:
     classifier: { name: abc, namespace: bad-namespace }
 services:
   - serviceName: order-processor
     config: |+
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: cde, namespace: namespace }
`

	exp := &model.TopicRegistrationRespDto{}
	errInParsing := errors.New("bad namespace")

	gomock.InOrder(
		kafkaInstanceServiceMock.EXPECT().
			UpsertKafkaInstanceDesignator(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, errInParsing).
			AnyTimes(),
		kafkaServiceMock.EXPECT().
			GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			AnyTimes(),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal(3, len(result))
	assert.Equal(
		model.ConfigMsResult{Status: "error", Error: "Error during applying aggregated config, error: 'configurator_service: server error for internal config of microservice', message: (Error during applying inner config for microservice with name '', err: bad namespace)", Data: nil},
		result[1].Result)
	assert.Equal("ok", result[0].Result.Status)
	assert.Equal("ok", result[2].Result.Status)
}

func TestSharedConfigRabbitV2Error(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
 version: v1
 namespace: namespace
 shared: |
   apiVersion: nc.maas.rabbit/v2
   kind: vhost
   spec:
       classifier:
          name: test
          namespace: namespace
       versionedEntities:
           exchanges:
           - name: test-exchange-2
 services:
   - serviceName: order-processor
     config: |+
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: cde, namespace: namespace }
`

	exp := &model.TopicRegistrationRespDto{}

	gomock.InOrder(
		kafkaServiceMock.EXPECT().
			GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.Error(err)
	assert.Equal(2, len(result))
	assert.Equal("Error during applying aggregated config, error: 'configurator_service: server error for internal config of microservice', message: (Error during applying rabbit inner config for microservice with name '', err: service name is empty for rabbit v2 config, bg is not possible if config is not linked to ms, you should not put rabbit v2 config to shared config)", err.Error())
}

func TestSharedConfigKafkaAndRabbitDesignator(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
 version: v1
 namespace: namespace
 shared: |
   apiVersion: nc.maas.kafka/v2
   kind: instance-designator
   spec:
       namespace: namespace
       defaultInstance: cpq-kafka-maas-test
       selectors:
       - classifierMatch:
           name: orders
         instance: cpq-kafka-maas-test
       - classifierMatch:
           tenantId: test-tenant
         instance: cpq-kafka-maas-test
   ---
   apiVersion: nc.maas.rabbit/v2
   kind: instance-designator
   spec:
       namespace: namespace
       defaultInstance: fdebdf39-4d65-4064-a1d1-23f4b34a38a2
       selectors:
       - classifierMatch:
           name: orders
         instance: fdebdf39-4d65-4064-a1d1-23f4b34a38a2
       - classifierMatch:
           tenantId: test-tenant
         instance: fdebdf39-4d65-4064-a1d1-23f4b34a38a2
 services:
   - serviceName: order-processor
     config: |+
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: abc, namespace: namespace }
   - serviceName: order-executor
     config: |+
       ---
       apiVersion: nc.maas.rabbit/v1
       kind: vhost
       spec:
           classifier:
             name: test
             namespace: namespace
`

	exp := &model.TopicRegistrationRespDto{}

	kafkaServiceMock.EXPECT().
		GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	kafkaInstanceServiceMock.EXPECT().
		UpsertKafkaInstanceDesignator(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	rabbitInstanceServiceMock.EXPECT().
		UpsertRabbitInstanceDesignator(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(nil).
		Times(1)
	kafkaServiceMock.EXPECT().
		GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(true, exp, nil).
		Times(1)
	rabbitServiceMock.EXPECT().
		ProcessExportedVhost(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	gomock.InOrder(
		rabbitServiceMock.EXPECT().
			GetOrCreateVhost(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, &model.VHostRegistration{}, nil).
			Times(1),
		rabbitServiceMock.EXPECT().
			GetConnectionUrl(gomock.Any(), gomock.Any()).
			Return("", nil).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, rabbitInstanceServiceMock, rabbitServiceMock, nil, kafkaServiceMock, nil, nil, nil)
	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.NoError(err)
	assert.Equal(4, len(result))

	assert.Equal(model.STATUS_OK, result[0].Result.Status)
}

func TestBaseNamespaceAndKafkaTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
 version: v1
 namespace: namespace-1
 base-namespace: baseNamespace
 services:
   - serviceName: order-processor
     config: |+
         apiVersion: nc.maas.kafka/v1
         kind: topic
         spec:
            classifier: { name: topic, namespace: namespace-1 }
`

	exp := &model.TopicRegistrationRespDto{}
	gomock.InOrder(
		compositeNamespaceManager.EXPECT().
			GetByNamespace(gomock.Any(), "baseNamespace").
			Return(nil, nil).
			Times(1),
		compositeNamespaceManager.EXPECT().
			Upsert(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetOrCreateTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, exp, nil).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "namespace-1"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	bgDomainService := mock_domain.NewMockBGDomainService(mockCtrl)
	bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Eq("baseNamespace")).Return(nil, nil).Times(1)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, bgDomainService, compositeNamespaceManager)

	result, err := service.ApplyConfigV2(ctxMock, cfg)

	assert.NoError(err)
	assert.Equal(1, len(result))

	assert.Equal(model.STATUS_OK, result[0].Result.Status)
}

func Test_applyKafkaTopicDefinition_ToTopicDefinition_failed(t *testing.T) {
	assert := _assert.New(t)

	name1 := "name-1"
	namespace1 := "namespace-1"
	replicationFactorValue := "invalid_ReplicationFactor_value"

	classifier := model.Classifier{}
	classifier.Name = name1
	classifier.Namespace = namespace1

	cfg := model.TopicRegistrationConfigReqDto{
		Spec: model.TopicRegistrationReqDto{
			Classifier:        classifier,
			ReplicationFactor: replicationFactorValue,
		},
	}

	configuratorService := DefaultConfiguratorService{}
	_, err := configuratorService.applyKafkaTopicDefinition(context.Background(),
		&cfg, namespace1, "kind")

	err, ok := err.(model.AggregateConfigError)
	assert.True(ok)
	assert.True(strings.ContainsAny(err.Error(), replicationFactorValue))
}

func Test_registry_orderedKinds(t *testing.T) {
	assert := _assert.New(t)
	r := newRegistry()
	firstKind := model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicTemplateDeleteKind}
	secondKind := model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TenantTopicDeleteKind}
	thirdKind := model.Kind{ApiVersion: KafkaConfigV2ApiVersion, Kind: TopicDeleteKind}

	r.add(FirstOrder+2, thirdKind, &KindProcessor{value: model.TopicDeleteConfig{}, handler: nil})
	r.add(FirstOrder+1, secondKind, &KindProcessor{value: model.TopicDeleteConfig{}, handler: nil})
	r.add(FirstOrder, firstKind, &KindProcessor{value: model.TopicDeleteConfig{}, handler: nil})

	orderedKinds := r.orderedKinds()
	assert.Equal(firstKind, orderedKinds[0])
	assert.Equal(secondKind, orderedKinds[1])
	assert.Equal(thirdKind, orderedKinds[2])
}

func TestApplyKafkaDeleteTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: test-ns
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v2
          kind: topic-delete
          spec:
              classifier: 
                  name: my-test
                  namespace: test-ns
`

	kafkaServiceMock.EXPECT().
		DeleteTopics(gomock.Any(), &model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "my-test",
				Namespace: "test-ns",
			},
		}).
		Times(1)
	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-ns"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, compositeNamespaceManager)
	_, err := service.ApplyConfigV2(ctxMock, cfg)
	assert.NoError(err)
}

func TestApplyKafkaDeleteTenantTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: test-ns
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v2
          kind: tenant-topic-delete
          spec:
              classifier: 
                  name: my-test
                  namespace: test-ns
`

	gomock.InOrder(
		kafkaServiceMock.EXPECT().
			DeleteTopicDefinition(
				gomock.Any(),
				gomock.Eq(&model.Classifier{Name: "my-test", Namespace: "test-ns"}),
			).
			Times(1),
		tenantServiceMock.EXPECT().
			GetTenantsByNamespace(gomock.Any(), gomock.Eq("test-ns")).
			Return([]model.Tenant{{ExternalId: "test-tenant"}}, nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			DeleteTopics(gomock.Any(), &model.TopicSearchRequest{
				Classifier: model.Classifier{Name: "my-test", TenantId: "test-tenant", Namespace: "test-ns"},
			}).
			Times(1),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-ns"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, tenantServiceMock, kafkaServiceMock, nil, nil, compositeNamespaceManager)
	_, err := service.ApplyConfigV2(ctxMock, cfg)
	assert.NoError(err)
}

func TestApplyKafkaDeleteTopicTemplate(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	assert := _assert.New(t)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec: 
  version: v1
  namespace: test-ns
  services: 
    - serviceName: order-processor
      config: |+
          apiVersion: nc.maas.kafka/v2
          kind: topic-template-delete
          spec:
              name: my-test
`

	kafkaServiceMock.EXPECT().
		DeleteTopicTemplate(gomock.Any(), "my-test", "test-ns").
		Times(1)
	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-ns"
	ctxMock := model.WithRequestContext(context.Background(), requestContext)

	service := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaServiceMock, nil, nil, compositeNamespaceManager)
	_, err := service.ApplyConfigV2(ctxMock, cfg)
	assert.NoError(err)
}

func TestKafkaTopicDefinitionsWarmup_TenantTopicDefinition(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	cfgFirstNamespace := `
---
apiVersion: nc.maas.kafka/v1
kind: tenant-topic
spec:
  classifier:
    name: topic-name
    namespace: first-ns
`

	cfgSecondNamespace := `
---
apiVersion: nc.maas.kafka/v1
kind: tenant-topic
spec:
 classifier:
   name: topic-name
   namespace: second-ns
`

	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		baseDao := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 5,
			DrMode:   dr.Active,
		})
		defer baseDao.Close()

		domainDao := domain.NewBGDomainDao(baseDao)
		requestContext := &model.RequestContext{Namespace: "first-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		_, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := kafka.NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		kafkaHelper := mock_kafka_helper.NewMockHelper(mockCtrl)
		instanceService := instance.NewKafkaInstanceService(instanceDao, kafkaHelper)

		eventBus := kafka.NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

		authService := mock_auth.NewMockAuthService(mockCtrl)
		kafkaService := kafka.NewKafkaService(sd, instanceService, kafkaHelper, auditService, nil, eventBus, authService)
		tenantServiceDao := tenant.NewTenantServiceDaoImpl(baseDao)
		tenantService := tenant.NewTenantService(tenantServiceDao, kafkaService)

		configuratorService := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, tenantService, kafkaService, nil, nil, nil)
		results, err := configuratorService.ApplyConfig(ctx, cfgFirstNamespace, "first-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		err = sd.Warmup(ctx, "first-ns", "second-ns")
		assert.NoError(t, err)

		requestContext = &model.RequestContext{Namespace: "second-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)

		results, err = configuratorService.ApplyConfig(ctx, cfgSecondNamespace, "second-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		topicDefinitions, err := kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, "first-ns", model.TopicDefinitionKindTenant)
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "topic-name", topicDefinitions[0].Classifier.Name)
		firstTopicDefId := topicDefinitions[0].Id

		topicDefinitions, err = kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, "second-ns", model.TopicDefinitionKindTenant)
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "topic-name", topicDefinitions[0].Classifier.Name)
		assert.Equal(t, firstTopicDefId, topicDefinitions[0].Id)
	})
}

func TestKafkaTenantTopic_ChangeMinNumPartition(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()
	req := `
[
	{
		"externalId" : "first"
	}
]
`

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		requestContext := &model.RequestContext{Namespace: "test-namespace"}
		ctx = model.WithRequestContext(context.Background(), requestContext)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		kafkaInstance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)

		kafkaHelper := mock_kafka_helper.NewMockHelper(mockCtrl)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), gomock.Any()).
			Return(false, nil).
			Times(1)
		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
				if *topic.NumPartitions != 10 {
					assert.Fail(t,
						"topic.NumPartitions MUST be equal 10",
						"topic.NumPartitions MUST be equal 10, but it is %d",
						*topic.NumPartitions,
					)
				}
				if *topic.MinNumPartitions != 0 {
					assert.Fail(t,
						"topic.NumPartitions MUST be equal 0",
						"topic.NumPartitions MUST be equal 0, but it is %d",
						*topic.MinNumPartitions,
					)
				}
				return &model.TopicRegistrationRespDto{Name: topic.Topic}, nil
			}).
			Times(1)
		kafkaHelper.EXPECT().
			UpdateTopicSettings(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, topic *model.TopicRegistrationRespDto) error {
				if *topic.RequestedSettings.NumPartitions != 0 {
					assert.Fail(t,
						"topic.RequestedSettings.NumPartitions MUST be equal 0",
						"topic.RequestedSettings.NumPartitions MUST be equal 0, but it is %d",
						*topic.RequestedSettings.NumPartitions,
					)
				}
				if *topic.RequestedSettings.MinNumPartitions != 2 {
					assert.Fail(t,
						"topic.RequestedSettings.MinNumPartitions MUST be equal 2",
						"topic.RequestedSettings.MinNumPartitions MUST be equal 2, but it is %d",
						*topic.RequestedSettings.MinNumPartitions,
					)
				}
				return nil
			}).
			Times(1)

		bgDomainService := kafka.NewMockBGDomainService(mockCtrl)
		bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		eventBus := kafka.NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().
			GetAllowedNamespaces(gomock.Any(), "test-namespace").
			Return([]string{"test-namespace"}, nil).
			AnyTimes()
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		tenantServiceDao := tenant.NewTenantServiceDaoImpl(baseDao)
		sd := kafka.NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })
		instanceService := instance.NewKafkaInstanceService(instanceDao, kafkaHelper)
		kafkaService := kafka.NewKafkaService(sd, instanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)
		tenantService := tenant.NewTenantService(tenantServiceDao, kafkaService)

		configuratorService := NewConfiguratorServiceV1(instanceService, nil, nil, tenantService, kafkaService, nil)
		_, err = configuratorService.ApplyKafkaTenantTopic(ctx, &model.TopicRegistrationConfigReqDto{
			Spec: model.TopicRegistrationReqDto{
				Name:          "test-topic",
				Classifier:    model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
				NumPartitions: 10,
			},
		}, "test-namespace")
		assert.NoError(t, err)

		tenants, err := tenantService.ApplyTenants(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, tenants, 1)

		_, err = configuratorService.ApplyKafkaTenantTopic(ctx, &model.TopicRegistrationConfigReqDto{
			Spec: model.TopicRegistrationReqDto{
				Name:             "test-topic",
				Classifier:       model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
				MinNumPartitions: 2,
			},
		}, "test-namespace")
		assert.NoError(t, err)
	})
}

func TestKafkaBG2_LazyTopicDefinition(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	cfgFirstNamespace := `
---
apiVersion: nc.maas.kafka/v1
kind: lazy-topic
spec:
  name: my-topic
  classifier:
    name: topic-name
    namespace: first-ns
`

	cfgSecondNamespace := `
---
apiVersion: nc.maas.kafka/v1
kind: lazy-topic
spec:
  name: my-topic
  classifier:
    name: topic-name
    namespace: second-ns
`

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		requestContext := &model.RequestContext{Namespace: "first-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		kafkaHelper := mock_kafka_helper.NewMockHelper(mockCtrl)
		kafkaInstance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), gomock.Any()).
			Return(false, nil).
			Times(1)
		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
				return &model.TopicRegistrationRespDto{Name: topic.Topic}, nil
			}).
			Times(1)

		_, err = instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := kafka.NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		bgDomainService := kafka.NewMockBGDomainService(mockCtrl)
		bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()
		eventBus := kafka.NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).MinTimes(1)
		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		instanceService := instance.NewKafkaInstanceService(instanceDao, kafkaHelper)
		kafkaService := kafka.NewKafkaService(sd, instanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		configuratorService := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaService, nil, nil, nil)
		results, err := configuratorService.ApplyConfig(ctx, cfgFirstNamespace, "first-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		err = sd.Warmup(ctx, "first-ns", "second-ns")
		assert.NoError(t, err)

		requestContext = &model.RequestContext{Namespace: "second-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)

		results, err = configuratorService.ApplyConfig(ctx, cfgSecondNamespace, "second-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		topicDefinitions, err := kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, "first-ns", model.TopicDefinitionKindLazy)
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "topic-name", topicDefinitions[0].Classifier.Name)
		firstTopicDefId := topicDefinitions[0].Id

		topicDefinitions, err = kafkaService.GetTopicDefinitionsByNamespaceAndKind(ctx, "second-ns", model.TopicDefinitionKindLazy)
		assert.NoError(t, err)
		assert.Len(t, topicDefinitions, 1)
		assert.Equal(t, "topic-name", topicDefinitions[0].Classifier.Name)
		assert.Equal(t, firstTopicDefId, topicDefinitions[0].Id)

		found, topic, err := kafkaService.GetOrCreateTopic(ctx, &model.TopicRegistration{
			Classifier: topicDefinitions[0].Classifier,
			Topic:      topicDefinitions[0].Name,
			Namespace:  topicDefinitions[0].Namespace,
		}, model.Fail)
		assert.NoError(t, err)
		assert.False(t, found)
		assert.NotNil(t, topic)
		assert.Equal(t, "my-topic", topic.Name)
	})
}

func TestKafkaBG2_TopicTemplate(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	cfgTopicTemplate := `
---
apiVersion: nc.maas.kafka/v1
kind: topic-template
pragma:
  on-entity-exists: merge
spec:
  name: test-topic
  numPartitions: 2
`

	cfgTopicFirstNs := `
---
apiVersion: nc.maas.kafka/v1
kind: topic
pragma: 
  on-entity-exists: merge 
spec: 
  classifier: 
    name: my-test-topic
    namespace: first-ns
  template: test-topic
`

	cfgTopicSecondNs := `
---
apiVersion: nc.maas.kafka/v1
kind: topic
pragma: 
  on-entity-exists: merge 
spec: 
  classifier: 
    name: my-test-topic
    namespace: second-ns
  template: test-topic
`

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		requestContext := &model.RequestContext{Namespace: "first-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		_, err := instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		kafkaHelper := mock_kafka_helper.NewMockHelper(mockCtrl)
		kafkaInstance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		authService := mock_auth.NewMockAuthService(mockCtrl)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), gomock.Any()).
			Return(false, nil).
			Times(1)
		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
				return &model.TopicRegistrationRespDto{Name: topic.Topic}, nil
			}).
			Times(1)
		kafkaHelper.EXPECT().
			GetTopicSettings(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		bgDomainDao := domain.NewBGDomainDao(baseDao)
		bgDomainService := domain.NewBGDomainService(bgDomainDao)
		sd := kafka.NewKafkaServiceDao(baseDao, bgDomainService.FindByNamespace)
		instanceService := instance.NewKafkaInstanceService(instanceDao, kafkaHelper)
		eventBus := kafka.NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		kafkaService := kafka.NewKafkaService(sd, instanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		configuratorService := NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, nil, kafkaService, nil, nil, nil)

		results, err := configuratorService.ApplyConfig(ctx, cfgTopicTemplate, "first-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		results, err = configuratorService.ApplyConfig(ctx, cfgTopicFirstNs, "first-ns")
		assertion.NoError(err)
		assertion.Equal(1, len(results))

		requestContext = &model.RequestContext{Namespace: "second-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)

		results, err = configuratorService.ApplyConfig(ctx, cfgTopicSecondNs, "second-ns")
		assertion.ErrorContains(err, "no topic template was found by name")

		err = kafkaService.Warmup(ctx, &domain.BGState{
			Origin: &domain.BGNamespace{
				Name:    "first-ns",
				State:   "active",
				Version: "v1",
			},
			Peer: &domain.BGNamespace{
				Name:    "second-ns",
				State:   "candidate",
				Version: "v2",
			},
			UpdateTime: time.Now(),
		})
		assert.NoError(t, err)

		results, err = configuratorService.ApplyConfig(ctx, cfgTopicSecondNs, "second-ns")
		assertion.NoError(err)
	})
}
