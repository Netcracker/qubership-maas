package tenant

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/monitoring"
	mock_auth "github.com/netcracker/qubership-maas/service/auth/mock"
	"github.com/netcracker/qubership-maas/service/configurator_service"
	mock_instance "github.com/netcracker/qubership-maas/service/instance/mock"
	"github.com/netcracker/qubership-maas/service/kafka"
	mock_kafka_helper "github.com/netcracker/qubership-maas/service/kafka/helper/mock"
	mock_tenant "github.com/netcracker/qubership-maas/service/tenant/mock"
	_ "github.com/proullon/ramsql/driver"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	assertion     *assert.Assertions
	mockCtrl      *gomock.Controller
	ctx           context.Context = nil
	mockTenantDao *mock_tenant.MockTenantServiceDao
	tenantService configurator_service.TenantService
	mockKafkaDao  *kafka.MockKafkaDao

	auditService *monitoring.MockAuditor
	kafkaHelper  *mock_kafka_helper.MockHelper

	//rabbitServiceMock	*mock_service.Mock
	kafkaServiceMock         *kafka.MockKafkaService
	kafkaInstanceServiceMock *mock_instance.MockKafkaInstanceService
)

func testInitializer(t *testing.T) {
	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	mockTenantDao = mock_tenant.NewMockTenantServiceDao(mockCtrl)
	mockKafkaDao = kafka.NewMockKafkaDao(mockCtrl)
	auditService = monitoring.NewMockAuditor(mockCtrl)
	kafkaHelper = mock_kafka_helper.NewMockHelper(mockCtrl)

	kafkaServiceMock = kafka.NewMockKafkaService(mockCtrl)
	tenantService = NewTenantService(mockTenantDao, kafkaServiceMock)
	kafkaInstanceServiceMock = mock_instance.NewMockKafkaInstanceService(mockCtrl)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(context.Background(), requestContext)
}

func TestApplyKafkaTenantTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	cfg := `
---
apiVersion: nc.maas.kafka/v1
kind: tenant-topic
spec:
  classifier:
    name: topic-name
    namespace: ns
  configs:
    flush.ms: '1004'
`
	confStr := "1004"
	topicDef := model.TopicDefinition{
		Classifier: &model.Classifier{
			Name:      "topic-name",
			Namespace: "ns",
		},
		Namespace: "ns",
		Name:      "topic-name",
		Instance:  "",
		Configs:   map[string]*string{"flush.ms": &confStr},
		Kind:      "tenant",
	}

	tenant := model.Tenant{
		Id:                 1,
		Namespace:          "ns",
		ExternalId:         "123",
		TenantPresentation: nil,
	}
	topic := model.TopicRegistrationRespDto{
		Classifier: model.Classifier{Name: "topic-name", Namespace: "ns"},
		Namespace:  "ns",
		Name:       "topic-name",
		Instance:   "",
		RequestedSettings: &model.TopicSettings{
			Configs: map[string]*string{"flush.ms": &confStr},
		},
		ActualSettings: &model.TopicSettings{
			Configs: map[string]*string{"flush.ms": &confStr},
		},
	}

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	resp := []model.SyncTenantsResp{
		{
			Tenant: tenant,
			Topics: []*model.TopicRegistrationRespDto{&topic},
		},
	}
	authService := mock_auth.NewMockAuthService(mockCtrl)
	kafkaInstanceServiceMock.EXPECT().GetById(gomock.Any(), gomock.Any()).AnyTimes()
	gomock.InOrder(
		mockKafkaDao.EXPECT().
			FindTopicDefinitions(gomock.Any(), gomock.Any()).
			Return([]model.TopicDefinition{topicDef}, nil).
			Times(1),
		mockKafkaDao.EXPECT().
			UpdateTopicDefinition(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		mockTenantDao.EXPECT().
			GetTenantsByNamespace(gomock.Any(), gomock.Any()).
			Return([]model.Tenant{tenant}, nil).
			Times(1),
		mockKafkaDao.EXPECT().
			FindTopicTemplateByNameAndNamespace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&model.TopicTemplate{}, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(gomock.Any()).
			Return(kafkaInstance, nil).
			Times(1),
		mockKafkaDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockKafkaDao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(true, nil).
			Times(1),
		mockKafkaDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		mockKafkaDao.EXPECT().
			InsertTopicRegistration(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&topic, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	bgDomainService := kafka.NewMockBGDomainService(mockCtrl)
	bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "ns"
	ctx = model.WithRequestContext(context.Background(), requestContext)

	eventBus := kafka.NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	kafkaService := kafka.NewKafkaService(mockKafkaDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	kafkaInstanceServiceMock = mock_instance.NewMockKafkaInstanceService(mockCtrl)
	service := configurator_service.NewConfiguratorService(kafkaInstanceServiceMock, nil, nil, tenantService, kafkaService, nil, nil, nil)
	results, err := service.ApplyConfig(ctx, cfg, "ns")

	log.InfoC(ctx, "%v", results)
	assertion.NoError(err)
	assertion.Equal(1, len(results))
	assertion.Equal(model.ConfigMsResult{Status: "ok", Error: "", Data: resp}, results[0].Result)
}

func TestApplyTenants(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	req := `
    [
        {
            "externalId" : "100"
        } , 
        {
            "externalId" : "200"
        }
    ]

`
	confStr := "1004"

	tenant := model.Tenant{
		Id:                 1,
		Namespace:          "ns",
		ExternalId:         "100",
		TenantPresentation: nil,
	}

	topic := model.TopicRegistrationRespDto{
		Classifier: model.Classifier{Name: "topic-name"},
		Namespace:  "ns",
		Name:       "topic-name",
		Instance:   "",
		RequestedSettings: &model.TopicSettings{
			Configs: map[string]*string{"flush.ms": &confStr},
		},
		ActualSettings: &model.TopicSettings{
			Configs: map[string]*string{"flush.ms": &confStr},
		},
	}

	gomock.InOrder(
		mockTenantDao.EXPECT().
			GetTenantsByNamespace(gomock.Any(), gomock.Any()).
			Return([]model.Tenant{tenant}, nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			GetTopicDefinitionsByNamespaceAndKind(gomock.Any(), gomock.Eq("ns"), gomock.Eq(model.TopicDefinitionKindTenant)).
			Return([]model.TopicDefinition{{
				Classifier: &model.Classifier{Name: "topic-name"},
				Namespace:  "ns",
				Name:       "topic-name",
				Instance:   "",
				Configs:    map[string]*string{"flush.ms": &confStr},
				Kind:       "tenant",
			}}, nil).
			Times(1),
		mockTenantDao.EXPECT().
			InsertTenant(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		kafkaServiceMock.EXPECT().
			CreateTopicByTenantTopic(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(&topic, nil).
			Times(1),
	)
	requestContext := &model.RequestContext{Namespace: "ns"}
	ctx = model.WithRequestContext(context.Background(), requestContext)

	service := NewTenantService(mockTenantDao, kafkaServiceMock)
	results, err := service.ApplyTenants(ctx, req)

	log.InfoC(ctx, "%v", results)
	assertion.NoError(err)
	assertion.Equal(1, len(results))
	assertion.Equal(model.SyncTenantsResp{
		Tenant: model.Tenant{
			Id:                 0,
			Namespace:          "ns",
			ExternalId:         "200",
			TenantPresentation: map[string]interface{}{"externalId": "200"},
		},
		Topics: []*model.TopicRegistrationRespDto{&topic},
	}, results[0])
}
