package kafka

import (
	"context"
	"errors"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-maas/dao"
	dbc "github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/eventbus"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/monitoring"
	"github.com/netcracker/qubership-maas/msg"
	mock_auth "github.com/netcracker/qubership-maas/service/auth/mock"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/instance"
	mock_instance "github.com/netcracker/qubership-maas/service/instance/mock"
	mock_kafka_helper "github.com/netcracker/qubership-maas/service/kafka/helper/mock"
	"github.com/netcracker/qubership-maas/service/tenant"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
	"sync"
	"testing"
	"time"
)

var (
	assertion       *assert.Assertions
	mockCtrl        *gomock.Controller
	ctx             context.Context = nil
	mockDao         *MockKafkaDao
	auditService    *monitoring.MockAuditor
	authService     *mock_auth.MockAuthService
	kafkaHelper     *mock_kafka_helper.MockHelper
	bgDomainService *MockBGDomainService

	//rabbitServiceMock	*mock_service.Mock
	kafkaInstanceServiceMock *mock_instance.MockKafkaInstanceService

	namespace = "test-namespace"
)

func testInitializer(t *testing.T) {
	ctx = context.Background()

	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	mockDao = NewMockKafkaDao(mockCtrl)
	auditService = monitoring.NewMockAuditor(mockCtrl)
	authService = mock_auth.NewMockAuthService(mockCtrl)
	kafkaHelper = mock_kafka_helper.NewMockHelper(mockCtrl)
	bgDomainService = NewMockBGDomainService(mockCtrl)

	kafkaInstanceServiceMock = mock_instance.NewMockKafkaInstanceService(mockCtrl)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(ctx, requestContext)
}

func TestKafkaService_CreateNewTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
	}
	failOnTopicExists := model.Fail

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testTopic",
		Classifier:        classifier,
		Namespace:         "test-namespace",
		Instance:          kafkaInstance.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	gomock.InOrder(
		bgDomainService.EXPECT().
			FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaInstance), gomock.Eq(topicReg.Topic)).
			Return(false, nil).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Eq("test_kafka")).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(false, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_CreateNewLazyTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testLazyTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testLazyTopic",
		Namespace:  "test-namespace",
	}
	failOnTopicExists := model.Fail

	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testLazyTopic",
		Classifier:        classifier,
		Namespace:         "test-namespace",
		Instance:          kafkaInstance.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	lazyTopicDef := model.TopicDefinition{
		Id:         1,
		Classifier: &classifier,
		Namespace:  "test-namespace",
		Name:       "test-namespace.testLazyTopic",
		Instance:   kafkaInstance.Id,
		Kind:       "lazy",
	}

	gomock.InOrder(
		mockDao.EXPECT().
			FindTopicDefinitions(eqCtx, gomock.Eq(TopicDefinitionSearch{Namespace: "test-namespace", Kind: "lazy"})).
			Return([]model.TopicDefinition{lazyTopicDef}, nil).
			Times(1),
		bgDomainService.EXPECT().
			FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			FindTopicTemplateByNameAndNamespace(eqCtx, gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		bgDomainService.EXPECT().
			FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(kafkaInstance.Id)).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaInstance), gomock.Eq(topicReg.Topic)).
			Return(false, nil).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Eq("test_kafka")).
		Times(1)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	found, registeredTopic, err := service.GetOrCreateLazyTopic(ctx, &classifier, failOnTopicExists)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "test-namespace.testLazyTopic", registeredTopic.Name)
}

func TestKafkaService_CleanupNamespace(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testLazyTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testLazyTopic",
		Namespace:  "test-namespace",
	}

	gomock.InOrder(
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, &model.TopicSearchRequest{Namespace: "test-namespace"}).
			Return([]*model.TopicRegistration{topicReg}, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Any()).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			DeleteTopicRegistration(eqCtx, topicReg, gomock.Any()).
			Return(nil),
		mockDao.EXPECT().
			DeleteTopicTemplatesByNamespace(eqCtx, classifier.Namespace).
			Return(nil),
		mockDao.EXPECT().
			DeleteTopicDefinitionsByNamespace(eqCtx, classifier.Namespace).
			Return(nil),
		kafkaInstanceServiceMock.EXPECT().
			DeleteKafkaInstanceDesignatorByNamespace(eqCtx, classifier.Namespace).
			Return(nil),
	)

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, nil)
	err := service.CleanupNamespace(ctx, classifier.Namespace)
	assert.NoError(t, err)
}

func TestKafkaService_GetDiscrepancyReport(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	topicRegFirst := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first", Namespace: "test-namespace", TenantId: "1"},
		Topic:      "test-namespace.first",
		Namespace:  "test-namespace",
	}

	topicRegSecond := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "second", Namespace: "test-namespace", TenantId: "2"},
		Topic:      "test-namespace.second",
		Namespace:  "test-namespace",
	}

	gomock.InOrder(
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, &model.TopicSearchRequest{Namespace: "test-namespace"}).
			Return([]*model.TopicRegistration{topicRegFirst, topicRegSecond}, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Any()).
			Return(kafkaInstance, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), "test-namespace.first").
			Return(true, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), "test-namespace.second").
			Return(false, nil).
			Times(1),
	)

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, nil)
	report, err := service.GetDiscrepancyReport(ctx, "test-namespace", nil)
	assert.NoError(t, err)
	assert.NotNil(t, report)

	assert.Equal(t, "test-namespace.first", report[0].Name)
	assert.Equal(t, model.StatusOk, report[0].Status)
	assert.Equal(t, model.Classifier{Name: "first", Namespace: "test-namespace", TenantId: "1"}, report[0].Classifier)

	assert.Equal(t, "test-namespace.second", report[1].Name)
	assert.Equal(t, model.StatusAbsent, report[1].Status)
	assert.Equal(t, model.Classifier{Name: "second", Namespace: "test-namespace", TenantId: "2"}, report[1].Classifier)
}

func TestKafkaService_CreateTopicDefinition(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}

	topicDef := model.TopicDefinition{
		Id:         1,
		Classifier: &classifier,
		Namespace:  "test-namespace",
		Name:       "test-namespace.testLazyTopic",
		Instance:   kafkaInstance.Id,
		Kind:       "lazy",
	}

	gomock.InOrder(
		mockDao.EXPECT().
			FindTopicDefinitions(eqCtx, gomock.Eq(TopicDefinitionSearch{Classifier: &classifier, Kind: "lazy"})).
			Return([]model.TopicDefinition{}, nil).
			Times(1),
		mockDao.EXPECT().
			InsertTopicDefinition(eqCtx, &topicDef).
			Return(nil).
			Times(1),
	)

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, nil)
	found, registeredTopic, err := service.GetOrCreateTopicDefinition(ctx, &topicDef)
	assert.NoError(t, err)
	assert.False(t, found)
	assert.Equal(t, "test-namespace.testLazyTopic", registeredTopic.Name)
	assert.Equal(t, "lazy", registeredTopic.Kind)
}

func TestKafkaService_GetExistingTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	realPartitionsNum := int32(5)
	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		TopicSettings: model.TopicSettings{
			NumPartitions: &realPartitionsNum,
		},
	}
	existingTopicReg := []*model.TopicRegistration{{
		Classifier:    &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:         "test-namespace.testTopic",
		Namespace:     "test-namespace",
		Instance:      kafkaInstance.Id,
		TopicSettings: model.TopicSettings{NumPartitions: &realPartitionsNum},
	}}

	failOnTopicExists := model.Fail

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
	expected := &model.TopicRegistrationRespDto{
		Addresses:   map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:        "test-namespace.testTopic",
		Classifier:  classifier,
		Namespace:   "test-namespace",
		Instance:    kafkaInstance.Id,
		InstanceRef: kafkaInstance,
		RequestedSettings: &model.TopicSettings{
			NumPartitions: &realPartitionsNum,
		},
		ActualSettings: &model.TopicSettings{},
	}

	gomock.InOrder(
		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(existingTopicReg, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(kafkaInstance.Id)).
			Return(kafkaInstance, nil).
			Times(1),
		kafkaHelper.EXPECT().
			GetTopicSettings(eqCtx, gomock.Any()).
			Return(nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(true, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_UpdateExistingTopic(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}
	oldPartitionsNum := int32(3)
	newPartitionsNum := int32(5)
	existingTopicReg := []*model.TopicRegistration{{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		Instance:   kafkaInstance.Id,
		TopicSettings: model.TopicSettings{
			NumPartitions: &oldPartitionsNum,
		},
	}}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		TopicSettings: model.TopicSettings{
			NumPartitions: &newPartitionsNum,
		},
	}

	failOnTopicExists := model.Fail

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
	expected := &model.TopicRegistrationRespDto{
		Addresses:   map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:        "test-namespace.testTopic",
		Classifier:  classifier,
		Namespace:   "test-namespace",
		Instance:    kafkaInstance.Id,
		InstanceRef: kafkaInstance,
		RequestedSettings: &model.TopicSettings{
			NumPartitions: &newPartitionsNum,
		},
		ActualSettings: &model.TopicSettings{},
	}

	gomock.InOrder(
		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(existingTopicReg, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(kafkaInstance.Id)).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			UpdateTopicRegistration(eqCtx, gomock.Eq(existingTopicReg[0]), gomock.Any()).
			Return(nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(true, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_UpdateExistingTopicWithRealDAO(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:           "test_kafka",
		Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Default:      true,
		MaasProtocol: model.Plaintext,
	}
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		newDao := dao.New(&dbc.Config{
			Addr:      tdb.Addr(),
			User:      tdb.Username(),
			Password:  tdb.Password(),
			Database:  tdb.DBName(),
			PoolSize:  3,
			DrMode:    dr.Active,
			CipherKey: configloader.GetKoanf().MustString("db.cipher.key"),
		})

		domainDao := domain.NewBGDomainDao(newDao)
		kafkaInstanceDao := instance.NewKafkaInstancesDao(newDao, domainDao)
		kafkaInstanceService := instance.NewKafkaInstanceServiceWithHealthChecker(
			kafkaInstanceDao,
			nil,
			func(kafkaInstance *model.KafkaInstance) error {
				return nil
			},
		)
		_, err := kafkaInstanceService.Register(ctx, kafkaInstance)
		assert.NoError(t, err)
		//Test data emulate a DB record before CreateReq field was added. The same as topicReg but without CreateReq and with oldPartitionsNum := int32(3)
		tdb.Gorm(t).Exec(`INSERT INTO "kafka_topics" ("classifier","topic","instance","namespace","externally_managed","num_partitions","min_num_partitions","replication_factor","replica_assignment","configs","dirty","create_req") VALUES ('{"name": "testTopic", "namespace": "test-namespace"}','test-namespace.testTopic','test_kafka','test-namespace',false,3,NULL,NULL,NULL,NULL,false, NULL)`)

		newPartitionsNum := int32(5)

		topicReg := &model.TopicRegistration{
			Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
			Topic:      "test-namespace.testTopic",
			Namespace:  "test-namespace",
			TopicSettings: model.TopicSettings{
				NumPartitions: &newPartitionsNum,
			},
			CreateReq: `{
    "name": "test-namespace.testTopic",
    "classifier": {
        "namespace": "test-namespace",
        "name": "testTopic"
    },
    "numPartitions": 5
}`,
		}

		classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
		expected := &model.TopicRegistrationRespDto{
			Addresses:   map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
			Name:        "test-namespace.testTopic",
			Classifier:  classifier,
			Namespace:   "test-namespace",
			Instance:    kafkaInstance.Id,
			InstanceRef: kafkaInstance,
			RequestedSettings: &model.TopicSettings{
				NumPartitions: &newPartitionsNum,
			},
			ActualSettings: &model.TopicSettings{},
		}

		gomock.InOrder(
			bgDomainService.EXPECT().
				FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
				Times(1),
			kafkaHelper.EXPECT().
				UpdateTopicSettings(gomock.Any(), gomock.Any()).
				Return(nil).
				Times(1),
		)
		auditService.EXPECT().
			AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
			Times(1)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		eventBus := NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		kafkaService := NewKafkaService(NewKafkaServiceDao(newDao, nil), kafkaInstanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		found, updatedTopic, err := kafkaService.GetOrCreateTopic(ctx, topicReg, model.Fail)
		assertion.NoError(err)
		assertion.True(found)
		assertion.Equal(expected, updatedTopic)
	})
}

func TestKafkaService_UpdateExistingTopicWithRealDAO_Multithreaded(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)
	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(base)
		bgDomainService := domain.NewBGDomainService(domainDao)
		kafkaInstanceDao := instance.NewKafkaInstancesDao(base, domainDao)
		kafkaInstanceService := instance.NewKafkaInstanceServiceWithHealthChecker(
			kafkaInstanceDao,
			nil,
			func(kafkaInstance *model.KafkaInstance) error {
				return nil
			},
		)
		_, err := kafkaInstanceService.Register(ctx, &model.KafkaInstance{
			Id:           "test_kafka",
			Addresses:    map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
			Default:      true,
			MaasProtocol: model.Plaintext,
		})
		assert.NoError(t, err)

		topicReg := &model.TopicRegistration{
			Classifier:    &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
			Topic:         "test-namespace.testTopic",
			Namespace:     "test-namespace",
			TopicSettings: model.TopicSettings{},
			CreateReq: `{
						"name": "test-namespace.testTopic",
						"classifier": {
							"namespace": "test-namespace",
							"name": "testTopic"
						}
					}`,
		}

		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil).
			AnyTimes()
		kafkaHelper.EXPECT().CreateTopic(gomock.Any(), gomock.Any()).Return(nil, nil).Times(1)
		kafkaHelper.EXPECT().GetTopicSettings(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		auditService.EXPECT().
			AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		eventBus := eventbus.NewEventBus(eventbus.NewEventbusDao(base))
		kafkaServiceDao := NewKafkaServiceDao(base, bgDomainService.FindByNamespace)
		kafkaService := NewKafkaService(kafkaServiceDao, kafkaInstanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		wg := sync.WaitGroup{}
		concurrencyLevel := 100
		for i := 0; i < concurrencyLevel; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, _, err = kafkaService.GetOrCreateTopic(ctx, topicReg, model.Fail)
				assertion.NoError(err)
			}()
		}
		wg.Wait()
	})
}

func TestKafkaService_UpdateExistingTopic_InstanceUpdate(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaDefaultInstance := &model.KafkaInstance{
		Id:        "default-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	kafkaNewInstance := &model.KafkaInstance{
		Id:        "new-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost.new:9092"}},
	}

	oldPartitionsNum := int32(3)
	newPartitionsNum := int32(5)
	existingTopicReg := []*model.TopicRegistration{{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		Instance:   kafkaDefaultInstance.Id,
		TopicSettings: model.TopicSettings{
			NumPartitions: &oldPartitionsNum,
		},
	}}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		Instance:   kafkaNewInstance.Id,
		TopicSettings: model.TopicSettings{
			NumPartitions: &newPartitionsNum,
		},
	}

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
	failOnTopicExists := model.Fail

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(topicReg.Instance)).
			Return(kafkaNewInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(existingTopicReg, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(existingTopicReg[0].Instance)).
			Return(kafkaDefaultInstance, nil).
			Times(1),
	)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	_, _, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists)
	assert.ErrorContainsf(t, err, fmt.Sprintf("topic with classifier '%s' already exists on '%s' instance", classifier, kafkaDefaultInstance.Id), "")
}

func TestKafkaService_UpdateExistingTopic_NoInstanceUpdateWithEmptyInstance(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaDefaultInstance := &model.KafkaInstance{
		Id:        "default-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	oldPartitionsNum := int32(3)
	newPartitionsNum := int32(5)
	existingTopicReg := []*model.TopicRegistration{{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		Instance:   kafkaDefaultInstance.Id,
		TopicSettings: model.TopicSettings{
			NumPartitions: &oldPartitionsNum,
		},
	}}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
		TopicSettings: model.TopicSettings{
			NumPartitions: &newPartitionsNum,
		},
	}

	failOnTopicExists := model.Fail

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}

	expected := &model.TopicRegistrationRespDto{
		Addresses:   map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:        "test-namespace.testTopic",
		Classifier:  classifier,
		Namespace:   "test-namespace",
		Instance:    kafkaDefaultInstance.Id,
		InstanceRef: kafkaDefaultInstance,
		RequestedSettings: &model.TopicSettings{
			NumPartitions: &newPartitionsNum,
		},
		ActualSettings: &model.TopicSettings{},
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaDefaultInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(existingTopicReg, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(existingTopicReg[0].Instance)).
			Return(kafkaDefaultInstance, nil).
			Times(1),
		mockDao.EXPECT().
			UpdateTopicRegistration(eqCtx, gomock.Eq(existingTopicReg[0]), gomock.Any()).
			Return(nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(true, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_CreateTopicAndMergeWithExistingTopicOnKafka(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
	}

	mergeOnTopicExists := model.Merge

	classifier := model.Classifier{Name: "testTopic", Namespace: "test-namespace"}
	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testTopic",
		Classifier:        classifier,
		Namespace:         "test-namespace",
		Instance:          kafkaInstance.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaInstance), gomock.Eq(topicReg.Topic)).
			Return(true, nil).
			Times(1),

		mockDao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),

		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Eq(topicReg), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, mergeOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(false, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_FailMergeWithExistingTopicOnKafka(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "test-namespace",
	}

	failOnTopicExists := model.Fail

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(kafkaInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaInstance), gomock.Eq(topicReg.Topic)).
			Return(true, nil).
			Times(1),
		mockDao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
	)

	// =======================================================
	// perform test
	// =======================================================
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists)
	assertion.Equal(false, found)
	assertion.Nil(registeredTopic)
	assertion.NotNil(err)
}

func TestKafkaService_GetTopicByClassifier(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	namespace := "test-namespace"
	ctx := model.WithRequestContext(context.Background(), &model.RequestContext{Namespace: namespace})

	eqCtx := gomock.Eq(ctx)
	kafkaInstance := &model.KafkaInstance{
		Id:        "test_kafka",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	classifier := model.Classifier{
		TenantId: "007",
		Name:     "test-name",
	}

	regMock := []*model.TopicRegistration{{
		Topic:      "test-namespace.007.test-name",
		Namespace:  "test-namespace",
		Classifier: &classifier,
		Instance:   kafkaInstance.Id,
	}}

	gomock.InOrder(
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *regMock[0].Classifier})).
			Return(regMock, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(kafkaInstance.Id)).
			Return(kafkaInstance, nil).
			Times(1),
		kafkaHelper.EXPECT().
			GetTopicSettings(eqCtx, gomock.Any()).
			Return(nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	kafkaService := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, nil, eventBus, authService)
	reg, err := kafkaService.GetTopicByClassifier(ctx, classifier)
	if err != nil {
		t.Fatalf("Failed: %v", err.Error())
	}
	assertion.Equal(namespace, reg.Namespace)
	assertion.Equal(classifier, reg.Classifier)
}

func TestKafkaTopicDeletion_onEmptyReqDoNotDelete(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	searchDeleteReq := &model.TopicSearchRequest{}

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	kafkaService := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, nil, eventBus, nil)
	result, err := kafkaService.DeleteTopics(ctx, searchDeleteReq)

	assertion.Nil(result)
	assertion.Equal(ErrSearchTopicAttemptWithEmptyCriteria, err)
}

func TestKafkaTopicDeletion_leaveRealTopicIntact(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		classifier := model.Classifier{Name: fmt.Sprintf("testTopic-%s", uuid.New()), Namespace: "test-namespace"}
		topicReg := &model.TopicRegistration{
			Classifier: &classifier,
			Namespace:  "test-namespace",
		}

		domainDao := domain.NewBGDomainDao(base)
		instanceManager := instance.NewKafkaInstanceServiceWithHealthChecker(
			instance.NewKafkaInstancesDao(base, domainDao),
			kafkaHelper,
			func(kafkaInstance *model.KafkaInstance) error {
				return nil
			},
		)
		instanceManager.Register(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"localhost:9092"}},
			Default:      true,
			MaasProtocol: "PLAINTEXT",
		})
		bgDomainService.EXPECT().
			FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).
			Times(1)
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil).
			Times(1)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1)

		eventBus := NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).AnyTimes()
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		// =======================================================
		// perform test
		// =======================================================
		service := NewKafkaService(NewKafkaServiceDao(base, nil), instanceManager, kafkaHelper, auditService, bgDomainService, eventBus, authService)
		found, _, err := service.GetOrCreateTopic(ctx, topicReg, model.Fail)

		assertion.NoError(err)
		assertion.Equal(false, found)

		searchDeleteReq := &model.TopicSearchRequest{
			Namespace:            "test-namespace",
			LeaveRealTopicIntact: true,
		}

		kafkaService := NewKafkaService(NewKafkaServiceDao(base, nil), instanceManager, kafkaHelper, auditService, nil, eventBus, nil)
		result, err := kafkaService.DeleteTopics(ctx, searchDeleteReq)

		assertion.NoError(err)
		assertion.Equal("test-namespace", result.DeletedSuccessfully[0].Namespace)
	})
}

func TestKafkaTopicSearch_onEmptyReq(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	searchDeleteReq := &model.TopicSearchRequest{}
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	kafkaService := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, nil, eventBus, nil)

	result, err := kafkaService.SearchTopics(ctx, searchDeleteReq)

	assertion.Nil(result)
	assertion.Equal(ErrSearchTopicAttemptWithEmptyCriteria, err)
}

func TestKafkaService_CreateNewTopicWithDesignatorSelector(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaInstanceFromDesig := &model.KafkaInstance{
		Id:        "selector-instance-designator",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	namespace := "test-namespace"
	tenantId := "test-tenant"
	defaultInstanceDesig := "default-designator-instance"
	classifier := model.Classifier{Name: "testTopic", Namespace: namespace, TenantId: tenantId}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace", TenantId: "test-tenant"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "namespace",
	}
	failOnTopicExists := model.Fail

	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testTopic",
		Classifier:        classifier,
		Namespace:         namespace,
		Instance:          kafkaInstanceFromDesig.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	designator := model.InstanceDesignatorKafka{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: &defaultInstanceDesig,
		InstanceSelectors: []*model.InstanceSelectorKafka{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{TenantId: tenantId},
				InstanceId:           kafkaInstanceFromDesig.Id,
				Instance:             kafkaInstanceFromDesig,
			},
		},
		DefaultInstance: nil,
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(&designator, nil).
			Times(1),
		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(eqCtx, gomock.Eq(&model.TopicSearchRequest{Classifier: *topicReg.Classifier})).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaInstanceFromDesig), gomock.Eq(topicReg.Topic)).
			Return(false, nil).
			Times(1),
		mockDao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(false, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_CreateNewTopicWithDesignatorDefault(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaSelectorInstanceFromDesig := &model.KafkaInstance{
		Id:        "selector-instance-designator",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	kafkaDefaultInstanceFromDesig := &model.KafkaInstance{
		Id:        "default-designator-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	namespace := "test-namespace"
	tenantId := "test-tenant"
	classifier := model.Classifier{Name: "testTopic", Namespace: namespace, TenantId: tenantId}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace", TenantId: "test-tenant"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "namespace",
	}
	failOnTopicExists := model.Fail

	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testTopic",
		Classifier:        classifier,
		Namespace:         namespace,
		Instance:          kafkaDefaultInstanceFromDesig.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	designator := model.InstanceDesignatorKafka{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: &kafkaDefaultInstanceFromDesig.Id,
		InstanceSelectors: []*model.InstanceSelectorKafka{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{TenantId: "wrong-tenant"},
				InstanceId:           kafkaSelectorInstanceFromDesig.Id,
				Instance:             kafkaSelectorInstanceFromDesig,
			},
		},
		DefaultInstance: kafkaDefaultInstanceFromDesig,
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(&designator, nil).
			Times(1),
		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
				Classifier: classifier,
			})).
			Return(nil, nil),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(kafkaDefaultInstanceFromDesig), gomock.Eq(topicReg.Topic)).
			Return(false, nil).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(false, found)
		assertion.Equal(expected, registeredTopic)
	}

}

func TestKafkaService_CreateNewTopicWithMaaSDefault(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaSelectorInstanceFromDesig := &model.KafkaInstance{
		Id:        "selector-instance-designator",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	MaaSDefaultInstance := &model.KafkaInstance{
		Id:        "maas-default-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	namespace := "test-namespace"
	tenantId := "test-tenant"
	classifier := model.Classifier{Name: "testTopic", Namespace: namespace, TenantId: tenantId}

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace", TenantId: "test-tenant"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "namespace",
	}
	failOnTopicExists := model.Fail

	expected := &model.TopicRegistrationRespDto{
		Addresses:         map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
		Name:              "test-namespace.testTopic",
		Classifier:        classifier,
		Namespace:         namespace,
		Instance:          MaaSDefaultInstance.Id,
		RequestedSettings: &model.TopicSettings{},
		ActualSettings:    &model.TopicSettings{},
	}

	designator := model.InstanceDesignatorKafka{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: nil,
		InstanceSelectors: []*model.InstanceSelectorKafka{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{TenantId: "wrong-tenant"},
				InstanceId:           kafkaSelectorInstanceFromDesig.Id,
				Instance:             kafkaSelectorInstanceFromDesig,
			},
		},
		DefaultInstance: nil,
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(&designator, nil).
			Times(1),
		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(MaaSDefaultInstance, nil).
			Times(1),
		mockDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(1),
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(eqCtx, gomock.Eq(MaaSDefaultInstance), gomock.Eq(topicReg.Topic)).
			Return(false, nil).
			Times(1),
		mockDao.EXPECT().
			FindTopicsBySearchRequest(gomock.Any(), gomock.Any()).
			Return(nil, nil),
		mockDao.EXPECT().
			InsertTopicRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(expected, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	if found, registeredTopic, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.Equal(false, found)
		assertion.Equal(expected, registeredTopic)
	}
}

func TestKafkaService_CreateNewTopicBothDesigAndInstanceError(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	kafkaSelectorInstanceFromDesig := &model.KafkaInstance{
		Id:        "selector-instance-designator",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	namespace := "test-namespace"

	topicReg := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "testTopic", Namespace: "test-namespace", TenantId: "test-tenant"},
		Topic:      "test-namespace.testTopic",
		Namespace:  "namespace",
		Instance:   "topic-instance",
	}
	failOnTopicExists := model.Fail

	designator := model.InstanceDesignatorKafka{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: nil,
		InstanceSelectors: []*model.InstanceSelectorKafka{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{TenantId: "wrong-tenant"},
				InstanceId:           kafkaSelectorInstanceFromDesig.Id,
				Instance:             kafkaSelectorInstanceFromDesig,
			},
		},
		DefaultInstance: nil,
	}

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	gomock.InOrder(

		bgDomainService.EXPECT().
			FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
			Times(1),
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(&designator, nil).
			Times(1),
	)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)
	found, _, err := service.GetOrCreateTopic(ctx, topicReg, failOnTopicExists)

	assertion.Equal(false, found)
	assertion.Error(err)
	assertion.ErrorIs(err, msg.Conflict)
}

func TestKafkaService_CreateNewTopicWildcardDesignator(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	asteriskInstance := &model.KafkaInstance{
		Id:        "asterisk-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	qmInstance := &model.KafkaInstance{
		Id:        "qm-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	fullNamedInstance := &model.KafkaInstance{
		Id:        "full-named-instance",
		Addresses: map[model.KafkaProtocol][]string{model.Plaintext: {"localhost:9092"}},
	}

	namespace := "test-namespace"

	asteriskTopicReg := &model.TopicRegistration{
		Classifier:        &model.Classifier{Name: "test-tenant", Namespace: "test-namespace"},
		Topic:             "test-namespace.testTopic",
		Namespace:         "namespace",
		ExternallyManaged: true,
	}

	qmTopicReg := &model.TopicRegistration{
		Classifier:        &model.Classifier{Name: "n-1", Namespace: "test-namespace"},
		Topic:             "test-namespace.testTopic",
		Namespace:         "namespace",
		ExternallyManaged: true,
	}

	fullNamedTopicReg := &model.TopicRegistration{
		Classifier:        &model.Classifier{Name: "full-named", Namespace: "test-namespace"},
		Topic:             "test-namespace.testTopic",
		Namespace:         "namespace",
		ExternallyManaged: true,
	}

	designator := model.InstanceDesignatorKafka{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: nil,
		InstanceSelectors: []*model.InstanceSelectorKafka{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{Name: "test-*"},
				InstanceId:           asteriskInstance.Id,
				Instance:             asteriskInstance,
			},
			{
				Id:                   2,
				InstanceDesignatorId: 2,
				ClassifierMatch:      model.ClassifierMatch{Name: "n-?"},
				InstanceId:           qmInstance.Id,
				Instance:             qmInstance,
			},
			{
				Id:                   3,
				InstanceDesignatorId: 3,
				ClassifierMatch:      model.ClassifierMatch{Name: "full-named"},
				InstanceId:           fullNamedInstance.Id,
				Instance:             fullNamedInstance,
			},
		},
		DefaultInstance: nil,
	}

	bgDomainService.EXPECT().
		FindByNamespace(eqCtx, gomock.Any()).Return(nil, nil).
		AnyTimes()
	kafkaInstanceServiceMock.EXPECT().
		GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
		Return(&designator, nil).
		AnyTimes()

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	// =======================================================
	// perform test
	// =======================================================

	service := NewKafkaService(mockDao, kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, nil)

	asteriskInstance, err := service.resolveKafkaInstance(ctx, *asteriskTopicReg.Classifier, asteriskTopicReg.Instance, namespace)
	assertion.NoError(err)
	assertion.Equal("asterisk-instance", asteriskInstance.Id)

	qmInstance, err = service.resolveKafkaInstance(ctx, *qmTopicReg.Classifier, qmTopicReg.Instance, namespace)
	assertion.NoError(err)
	assertion.Equal("qm-instance", qmInstance.Id)

	fullNamedInstance, err = service.resolveKafkaInstance(ctx, *fullNamedTopicReg.Classifier, fullNamedTopicReg.Instance, namespace)
	assertion.NoError(err)
	assertion.Equal("full-named-instance", fullNamedInstance.Id)
}

func TestKafkaService_WatchTopicCreate(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(base)
		// direct using dao to avoid kafka instance health checking as it done on service level
		kafkaInstanceDao := instance.NewKafkaInstancesDao(base, domainDao)
		instanceReg, err := kafkaInstanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"localhost:9092"}},
			Default:      true,
			MaasProtocol: "PLAINTEXT",
		})
		assert.NoError(t, err)

		eqCtx := gomock.Eq(ctx)
		kafkaInstanceServiceMock.EXPECT().
			GetKafkaInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
			Return(nil, nil).
			MinTimes(1)

		kafkaInstanceServiceMock.EXPECT().
			GetDefault(eqCtx).
			Return(instanceReg, nil).
			MinTimes(1)

		kafkaInstanceServiceMock.EXPECT().
			GetById(gomock.Any(), gomock.Eq("default")).
			Return(instanceReg, nil).
			MinTimes(1)

		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil).
			MinTimes(1)

		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			Times(2)

		kafkaHelper.EXPECT().
			GetTopicSettings(gomock.Any(), gomock.Any()).
			Return(nil).
			MinTimes(1)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			MinTimes(1)

		bgDomainService.EXPECT().
			FindByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			MinTimes(1)

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		// =======================================================
		// perform test
		// =======================================================
		eventBus := eventbus.NewEventBus(eventbus.NewEventbusDao(base))
		assert.NoError(t, eventBus.Start(ctx))
		service := NewKafkaService(NewKafkaServiceDao(base, nil), kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		classifier1 := model.Classifier{
			Name:      fmt.Sprintf("orders-%s", uuid.New()),
			Namespace: "test-namespace",
		}
		classifier2 := model.Classifier{
			Name:      fmt.Sprintf("invoices-%s", uuid.New()),
			Namespace: "test-namespace",
		}

		// emulate that first topic already created before watch will start
		_, _, err = service.GetOrCreateTopic(ctx, &model.TopicRegistration{Topic: "orders", Classifier: &classifier1}, model.Fail)
		assert.NoError(t, err)

		wg := testharness.NewSemaphore(t)
		go func() {
			{
				result, err := service.WatchTopicsCreate(ctx, []model.Classifier{classifier1, classifier2}, 5*time.Second)
				assert.NoError(t, err)
				assert.Equal(t, 1, len(result))
				assert.Equal(t, classifier1.ToJsonString(), result[0].Classifier.ToJsonString())
				wg.Notify("watch1-finished")
			}

			{
				result, err := service.WatchTopicsCreate(ctx, []model.Classifier{classifier2}, 5*time.Second)
				assert.NoError(t, err)
				assert.Equal(t, 1, len(result))
				assert.Equal(t, classifier2.ToJsonString(), result[0].Classifier.ToJsonString())
				wg.Notify("watch2-finished")
			}
		}()

		wg.Await("watch1-finished", 5*time.Second)

		utils.CancelableSleep(ctx, 1*time.Second)
		fmt.Println(">>>> Create topic")
		found, _, err := service.GetOrCreateTopic(ctx, &model.TopicRegistration{Topic: "invoices", Classifier: &classifier2}, model.Fail)
		assertion.NoError(err)
		assertion.False(found)

		wg.Await("watch2-finished", 10*time.Second)
	})
}

func TestKafkaService_WatchTopicCreateTimeout(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		classifier := model.Classifier{
			Name:      fmt.Sprintf("testTopic%s", uuid.New()),
			Namespace: "test-namespace",
		}

		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()

		dbao := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 3,
			DrMode:   dr.Active,
		})
		defer dbao.Close()

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Any()).
			Return(&domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"}, nil).
			Times(1)

		// =======================================================
		// perform test
		// =======================================================
		eventBus := eventbus.NewEventBus(eventbus.NewEventbusDao(dbao))
		assert.NoError(t, eventBus.Start(ctx))
		service := NewKafkaService(NewKafkaServiceDao(dbao, nil), kafkaInstanceServiceMock, kafkaHelper, auditService, bgDomainService, eventBus, authService)

		finds, err := service.WatchTopicsCreate(ctx, []model.Classifier{classifier}, 1*time.Second)
		assert.NoError(t, err)
		assert.Equal(t, 0, len(finds))
	})
}

func TestKafkaServiceImpl_SyncAllTopicsToKafka(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dao := NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	kafkaService := NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, authService)
	firstTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "first",
	}
	secondTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "second-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "second",
	}
	thirdTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "third-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "third",
	}
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Namespace: namespace,
		})).
		Return([]*model.TopicRegistration{firstTopicRegistration, secondTopicRegistration, thirdTopicRegistration}, nil)

	instanceService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("first")).
		Return(true, nil)
	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("second")).
		Return(true, errors.New("some error"))
	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("third")).
		Return(false, nil)

	kafkaHelperMock.EXPECT().
		CreateTopic(gomock.Any(), gomock.Eq(thirdTopicRegistration)).
		Return(nil, nil)

	report, err := kafkaService.SyncAllTopicsToKafka(ctx, namespace)
	assert.NoError(t, err)

	assert.Equal(t, 3, len(report))

	assert.Equal(t, firstTopicRegistration.Topic, report[0].Name)
	assert.Equal(t, model.SyncStatusExists, report[0].Status)
	assert.Empty(t, report[0].ErrMsg)
	assert.Equal(t, firstTopicRegistration.Classifier, &report[0].Classifier)

	assert.Equal(t, secondTopicRegistration.Topic, report[1].Name)
	assert.Equal(t, model.SyncStatusError, report[1].Status)
	assert.Equal(t, "some error", report[1].ErrMsg)
	assert.Equal(t, secondTopicRegistration.Classifier, &report[1].Classifier)

	assert.Equal(t, thirdTopicRegistration.Topic, report[2].Name)
	assert.Equal(t, model.SyncStatusAdded, report[2].Status)
	assert.Empty(t, report[2].ErrMsg)
	assert.Equal(t, thirdTopicRegistration.Classifier, &report[2].Classifier)
}

func TestKafkaServiceImpl_SyncTopicToKafka_Exists(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dao := NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	kafkaService := NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, authService)
	firstTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "first",
	}

	classifier := model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"}

	instanceService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: classifier,
		})).
		Return([]*model.TopicRegistration{firstTopicRegistration}, nil)

	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("first")).
		Return(true, nil)

	report, err := kafkaService.SyncTopicToKafka(ctx, classifier)
	assert.NoError(t, err)

	assert.Equal(t, firstTopicRegistration.Topic, report.Name)
	assert.Equal(t, model.SyncStatusExists, report.Status)
	assert.Empty(t, report.ErrMsg)
	assert.Equal(t, firstTopicRegistration.Classifier, &report.Classifier)
}

func TestKafkaServiceImpl_SyncTopicToKafka_Added(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dao := NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	kafkaService := NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, authService)
	firstTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "first",
	}

	classifier := model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"}

	instanceService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: classifier,
		})).
		Return([]*model.TopicRegistration{firstTopicRegistration}, nil)

	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("first")).
		Return(false, nil)

	kafkaHelperMock.EXPECT().
		CreateTopic(gomock.Any(), gomock.Eq(firstTopicRegistration)).
		Return(nil, nil)

	report, err := kafkaService.SyncTopicToKafka(ctx, classifier)
	assert.NoError(t, err)

	assert.Equal(t, firstTopicRegistration.Topic, report.Name)
	assert.Equal(t, model.SyncStatusAdded, report.Status)
	assert.Empty(t, report.ErrMsg)
	assert.Equal(t, firstTopicRegistration.Classifier, &report.Classifier)
}

func TestKafkaServiceImpl_SyncTopicToKafka_Error(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dao := NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	kafkaService := NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, authService)
	firstTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "first",
	}

	classifier := model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"}

	instanceService.EXPECT().
		GetById(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: classifier,
		})).
		Return([]*model.TopicRegistration{firstTopicRegistration}, nil)

	kafkaHelperMock.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Eq("first")).
		Return(true, errors.New("some error"))

	_, err := kafkaService.SyncTopicToKafka(ctx, classifier)
	assert.Error(t, err)
}

func TestKafkaServiceImpl_SyncTopicToKafka_NotFound(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	dao := NewMockKafkaDao(mockCtrl)
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	kafkaHelperMock := mock_kafka_helper.NewMockHelper(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	authService := mock_auth.NewMockAuthService(mockCtrl)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	kafkaService := NewKafkaService(dao, instanceService, kafkaHelperMock, auditService, nil, eventBus, authService)
	firstTopicRegistration := &model.TopicRegistration{
		Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"},
		Topic:      "first",
	}

	classifier := model.Classifier{Name: "first-test-topic", Namespace: "test-ns", TenantId: "42"}

	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: classifier,
		})).
		Return([]*model.TopicRegistration{}, nil)

	report, err := kafkaService.SyncTopicToKafka(ctx, classifier)
	assert.NoError(t, err)

	assert.Equal(t, model.SyncStatusNotFound, report.Status)
	assert.Empty(t, report.ErrMsg)
	assert.Equal(t, firstTopicRegistration.Classifier, &report.Classifier)
}

func TestKafkaTopicDefinitionsWarmup_TenantActivation(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()
	req := `
    [
        {
            "externalId" : "first"
        } , 
        {
            "externalId" : "second"
        }
    ]
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

		kafkaInstance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		classifier := model.Classifier{
			Name:      "first",
			Namespace: "first-ns",
		}
		err = sd.InsertTopicDefinition(ctx, &model.TopicDefinition{
			Classifier: &classifier,
			Kind:       model.TopicDefinitionKindTenant,
		})
		assert.NoError(t, err)

		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		kafkaHelper.EXPECT().
			DoesTopicExistOnKafka(gomock.Any(), gomock.Eq(kafkaInstance), gomock.Any()).
			Return(false, nil).
			AnyTimes()
		kafkaHelper.EXPECT().
			CreateTopic(gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, topic *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
				return &model.TopicRegistrationRespDto{Name: topic.Topic}, nil
			}).
			AnyTimes()
		kafkaHelper.EXPECT().
			GetTopicSettings(gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		bgDomainService := NewMockBGDomainService(mockCtrl)
		bgDomainService.EXPECT().FindByNamespace(gomock.Any(), gomock.Any()).Return(nil, nil).AnyTimes()

		eventBus := NewMockEventBus(mockCtrl)
		eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
		eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).MinTimes(1)

		authService := mock_auth.NewMockAuthService(mockCtrl)
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		instanceService := instance.NewKafkaInstanceService(instanceDao, kafkaHelper)
		kafkaService := NewKafkaService(sd, instanceService, kafkaHelper, auditService, bgDomainService, eventBus, authService)
		tenantServiceDao := tenant.NewTenantServiceDaoImpl(baseDao)
		tenantService := tenant.NewTenantService(tenantServiceDao, kafkaService)

		tenants, err := tenantService.ApplyTenants(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, tenants, 2)
		assert.NotNil(t, tenants[0].Topics[0])
		assert.NotNil(t, tenants[1].Topics[0])

		err = sd.Warmup(ctx, "first-ns", "second-ns")
		assert.NoError(t, err)

		requestContext = &model.RequestContext{Namespace: "second-ns"}
		ctx = model.WithRequestContext(context.Background(), requestContext)

		tenantsFromSecond, err := tenantService.ApplyTenants(ctx, req)
		assert.NoError(t, err)
		assert.Len(t, tenants, 2)
		assert.Equal(t, tenants[0].Topics[0].Name, tenantsFromSecond[0].Topics[0].Name)
		assert.Equal(t, tenants[1].Topics[0].Name, tenantsFromSecond[1].Topics[0].Name)
	})
}

func TestResolveTopicName_Defaults(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	bgDomain := NewMockBGDomainService(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	kafkaService := NewKafkaService(nil, nil, nil, nil, bgDomain, eventBus, nil)

	bgDomain.EXPECT().
		FindByNamespace(gomock.Any(), gomock.Eq("cloud-core")).
		Return(nil, nil).
		Times(1)

	name, err := kafkaService.resolveTopicName(ctx, "", model.Classifier{Name: "orders", Namespace: "cloud-core"}, false)
	assert.NoError(t, err)
	assert.Equal(t, "maas.cloud-core.orders", name)
}

func TestResolveTopicName_DefaultsWithTenant(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	bgDomain := NewMockBGDomainService(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)

	kafkaService := NewKafkaService(nil, nil, nil, nil, bgDomain, eventBus, nil)

	bgDomain.EXPECT().
		FindByNamespace(gomock.Any(), gomock.Eq("cloud-core")).
		Return(nil, nil).
		Times(1)

	name, err := kafkaService.resolveTopicName(ctx, "", model.Classifier{Name: "orders", TenantId: "my-tenant", Namespace: "cloud-core"}, false)
	assert.NoError(t, err)
	assert.Equal(t, "maas.cloud-core.my-tenant.orders", name)
}

func TestResolveTopicName_Bg2Domain(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	bgDomain := NewMockBGDomainService(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	kafkaService := NewKafkaService(nil, nil, nil, nil, bgDomain, eventBus, nil)

	bgDomain.EXPECT().
		FindByNamespace(gomock.Any(), gomock.Eq("cloud-core-sec")).
		Return(&domain.BGNamespaces{Origin: "cloud-core-pri"}, nil).
		Times(1)

	name, err := kafkaService.resolveTopicName(ctx, "", model.Classifier{Name: "orders", Namespace: "cloud-core-sec"}, false)
	assert.NoError(t, err)
	assert.Equal(t, "maas.cloud-core-pri.orders", name)
}

func TestResolveTopicName_Templating(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	bgDomain := NewMockBGDomainService(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	kafkaService := NewKafkaService(nil, nil, nil, nil, bgDomain, eventBus, nil)

	bgDomain.EXPECT().
		FindByNamespace(gomock.Any(), gomock.Eq("cloud-core")).
		Return(nil, nil).
		Times(2)

	name, err := kafkaService.resolveTopicName(ctx, "{{name}}.{{namespace}}.{{tenantId}}", model.Classifier{Name: "orders", Namespace: "cloud-core", TenantId: "test-tenant"}, false)
	assert.NoError(t, err)
	assert.Equal(t, "orders.cloud-core.test-tenant", name)

	name, err = kafkaService.resolveTopicName(ctx, "%name%.%namespace%.%tenantId%", model.Classifier{Name: "orders", Namespace: "cloud-core", TenantId: "test-tenant"}, false)
	assert.NoError(t, err)
	assert.Equal(t, "orders.cloud-core.test-tenant", name)
}

func TestKafkaServiceImpl_GetOrCreateTopicDefinition(t *testing.T) {
	testInitializer(t)

	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		dbao := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 3,
			DrMode:   dr.Active,
		})
		defer dbao.Close()

		classifier := model.Classifier{
			Name:      "test",
			Namespace: "test-namespace",
		}
		err := dbao.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
			return conn.Create(&model.TopicDefinition{
				Classifier: &classifier,
			}).Error
		})
		assert.NoError(t, err)

		sd := NewKafkaServiceDao(dbao, func(context.Context, string) (*domain.BGNamespaces, error) { return nil, nil })

		kafkaService := NewKafkaService(sd, nil, kafkaHelper, auditService, nil, nil, nil)

		_, _, err = kafkaService.GetOrCreateTopicDefinition(ctx, &model.TopicDefinition{Classifier: &classifier})
		assert.NoError(t, err)
	})
}

func TestKafkaServiceImpl_DestroyDomain_Ok(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	ctx = context.Background()
	dao := NewMockKafkaDao(mockCtrl)

	sd := NewKafkaService(dao, nil, nil, nil, nil, nil, nil)
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Namespace: "core-dev"})).
		Return(nil, nil).
		Times(1)
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Namespace: "core-dev-peer"})).
		Return(nil, nil).
		Times(1)

	err := sd.DestroyDomain(ctx, &domain.BGNamespaces{
		Origin: "core-dev",
		Peer:   "core-dev-peer",
	})

	assert.NoError(t, err)
}

func TestKafkaServiceImpl_DestroyDomain_Conflict(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	ctx = context.Background()
	dao := NewMockKafkaDao(mockCtrl)

	sd := NewKafkaService(dao, nil, nil, nil, nil, nil, nil)
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Namespace: "core-dev"})).
		Return([]*model.TopicRegistration{{}}, nil).
		Times(1)

	err := sd.DestroyDomain(ctx, &domain.BGNamespaces{
		Origin: "core-dev",
		Peer:   "core-dev-peer",
	})

	assert.ErrorIs(t, err, msg.Conflict)
}

func TestKafkaServiceImpl_DestroyDomain_DBError(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	ctx = context.Background()
	dao := NewMockKafkaDao(mockCtrl)

	sd := NewKafkaService(dao, nil, nil, nil, nil, nil, nil)
	dao.EXPECT().
		FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Namespace: "core-dev"})).
		Return(nil, fmt.Errorf("some database error")).
		Times(1)

	err := sd.DestroyDomain(ctx, &domain.BGNamespaces{
		Origin: "core-dev",
		Peer:   "core-dev-peer",
	})

	assert.ErrorContains(t, err, "some database error")
}

func TestConnectionContention(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		domainDao := domain.NewBGDomainDao(baseDao)
		instanceDao := instance.NewKafkaInstancesDao(baseDao, domainDao)
		instanceDao.InsertInstanceRegistration(ctx, &model.KafkaInstance{
			Id:           "default",
			Addresses:    map[model.KafkaProtocol][]string{"PLAINTEXT": {"kafka:9092"}},
			MaasProtocol: "PLAINTEXT",
		})

		instance, err := instanceDao.GetInstanceById(ctx, "default")
		assert.NoError(t, err)
		sd := NewKafkaServiceDao(baseDao, func(context.Context, string) (*domain.BGNamespaces, error) {
			return &domain.BGNamespaces{"primary", "secondary", ""}, nil
		})

		reg := model.TopicRegistration{
			Classifier:  &model.Classifier{Name: "a", Namespace: "primary"},
			Instance:    "default",
			Namespace:   "primary",
			InstanceRef: instance,
		}

		_, err = sd.InsertTopicRegistration(ctx, &reg, func(reg *model.TopicRegistration) (*model.TopicRegistrationRespDto, error) {
			return reg.ToResponseDto(), nil
		})

		wg := sync.WaitGroup{}
		for i := 0; i < 100; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				_, err := sd.FindTopicsBySearchRequest(ctx, &model.TopicSearchRequest{Classifier: *reg.Classifier})
				assert.NoError(t, err)
			}()
		}

		wg.Wait()
		assert.NoError(t, err)
	})
}

func TestKafkaServiceImpl_copyVersionedTopicsToPeer(t *testing.T) {
	testInitializer(t)
	trueVal := true
	mockCtrl := gomock.NewController(t)
	requestContext := &model.RequestContext{}
	requestContext.Namespace = "origin-namespace"
	reqCtx := model.WithRequestContext(context.Background(), requestContext)
	dao := NewMockKafkaDao(mockCtrl)
	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()
	instanceService := mock_instance.NewMockKafkaInstanceService(mockCtrl)
	instanceService.EXPECT().GetById(gomock.Any(), gomock.Any()).Return(&model.KafkaInstance{
		Id:      "42",
		Default: true,
	}, nil).AnyTimes()

	instanceService.EXPECT().
		GetKafkaInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()

	dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
		Namespace: "origin-namespace",
		Versioned: &trueVal,
	})).
		Return([]*model.TopicRegistration{
			{
				Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "origin-namespace"},
				Topic:      "first",
				CreateReq:  "",
			},
		}, nil)

	bgState := domain.BGState{
		ControllerNamespace: "controller-namespace",
		Origin: &domain.BGNamespace{
			Name:    "origin-namespace",
			State:   "active",
			Version: "v1",
		},
		Peer: &domain.BGNamespace{
			Name:    "peer-namespace",
			State:   "candidate",
			Version: "v2",
		},
		UpdateTime: time.Now(),
	}
	kafkaHelper.EXPECT().
		GetTopicSettings(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
	kafkaHelper.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil).
		AnyTimes()
	auditService.EXPECT().
		AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	sd := NewKafkaService(dao, instanceService, kafkaHelper, auditService, nil, eventBus, authService)
	err := sd.copyVersionedTopics(ctx, bgState.Origin.Name, bgState.Peer.Name)
	assert.ErrorIs(t, err, msg.BadRequest)

	dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
		Namespace: "origin-namespace",
		Versioned: &trueVal,
	})).
		Return([]*model.TopicRegistration{
			{
				Classifier: &model.Classifier{Name: "first-test-topic", Namespace: "origin-namespace"},
				Topic:      "first",
				CreateReq:  "{\"name\":\"test\",\"classifier\":{\"name\":\"first-test-topic\",\"namespace\":\"origin-namespace\"},\"instance\":\"test-instance\",\"versioned\":true}",
			},
		}, nil)
	err = sd.copyVersionedTopics(ctx, bgState.Origin.Name, bgState.Peer.Name)
	assert.Error(t, err)
	assert.ErrorIs(t, err, msg.BadRequest)
	assert.ErrorContains(t, err, "topic's name 'test' must contain {{namespace}} placeholder")

	dao.EXPECT().
		WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, lock string, f func(context.Context) error) { _ = f(ctx) }).
		AnyTimes()
	firstTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "first-test-topic", Namespace: "origin-namespace"},
		Topic:         "first",
		Instance:      "42",
		TopicSettings: model.TopicSettings{Versioned: true},
		CreateReq:     "{\"name\":\"%namespace%-test\",\"classifier\":{\"name\":\"first-test-topic\",\"namespace\":\"origin-namespace\"},\"instance\":\"test-instance\",\"versioned\":true}",
	}
	secondTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "second-test-topic", Namespace: "origin-namespace"},
		Topic:         "first",
		Instance:      "42",
		TopicSettings: model.TopicSettings{Versioned: true},
		CreateReq:     "{\"name\":\"%namespace%-test\",\"classifier\":{\"name\":\"second-test-topic\",\"namespace\":\"origin-namespace\"},\"instance\":\"test-instance\",\"versioned\":true}",
	}
	gomock.InOrder(
		dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Namespace: "origin-namespace",
			Versioned: &trueVal,
		})).
			Return([]*model.TopicRegistration{&firstTopicReg, &secondTopicReg}, nil),

		dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "first-test-topic",
				Namespace: "peer-namespace",
			},
		})).
			Return([]*model.TopicRegistration{}, nil),

		dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Topic:     "peer-namespace-test",
			Namespace: "peer-namespace",
		})).
			Return([]*model.TopicRegistration{}, nil),

		dao.EXPECT().
			InsertTopicRegistration(gomock.Any(), &firstTopicReg, gomock.Any()).
			Return(nil, nil).
			Times(1),

		dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Classifier: model.Classifier{
				Name:      "second-test-topic",
				Namespace: "peer-namespace",
			},
		})).
			Return([]*model.TopicRegistration{}, nil),

		dao.EXPECT().FindTopicsBySearchRequest(gomock.Any(), gomock.Eq(&model.TopicSearchRequest{
			Topic:     "peer-namespace-test",
			Namespace: "peer-namespace",
		})).
			Return([]*model.TopicRegistration{}, nil),

		dao.EXPECT().
			InsertTopicRegistration(gomock.Any(), &secondTopicReg, gomock.Any()).
			Return(nil, nil).
			Times(1),
	)
	err = sd.copyVersionedTopics(reqCtx, bgState.Origin.Name, bgState.Peer.Name)
	assert.NoError(t, err)
}

func TestKafkaServiceImpl_Warmup(t *testing.T) {
	testInitializer(t)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(context.Background(), requestContext)

	mockCtrl := gomock.NewController(t)

	dao := NewMockKafkaDao(mockCtrl)

	eventBus := NewMockEventBus(mockCtrl)
	eventBus.EXPECT().AddListener(gomock.Any()).Times(1)
	eventBus.EXPECT().Broadcast(gomock.Any(), gomock.Any(), gomock.Any()).AnyTimes()

	kafkaService := NewKafkaService(dao, kafkaInstanceServiceMock, kafkaHelper, nil, nil, eventBus, authService)

	expectedFirstTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "first-topic", Namespace: "peer"},
		Topic:         "peer-first-topic",
		Instance:      "first",
		Namespace:     "peer",
		TopicSettings: model.TopicSettings{Versioned: true},
		InstanceRef: &model.KafkaInstance{
			Id: "first",
		},
		CreateReq: `{
						"name": "{{namespace}}-first-topic",
						"classifier": {
							"namespace": "origin",
							"name": "first-topic"
						},
						"versioned": true
					}`,
	}
	expectedSecondTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "second-topic", Namespace: "peer"},
		Topic:         "peer-second-topic",
		Instance:      "first",
		Namespace:     "peer",
		TopicSettings: model.TopicSettings{Versioned: true},
		InstanceRef: &model.KafkaInstance{
			Id: "first",
		},
		CreateReq: `{
						"name": "{{namespace}}-second-topic",
						"classifier": {
							"namespace": "origin",
							"name": "second-topic"
						},
						"versioned": true
					}`,
	}

	dao.EXPECT().
		WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
		Do(func(ctx context.Context, lock string, f func(ctx context.Context) error) { _ = f(ctx) }).
		AnyTimes()
	kafkaHelper.EXPECT().
		DoesTopicExistOnKafka(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil).
		AnyTimes()
	kafkaInstanceServiceMock.EXPECT().
		GetKafkaInstanceDesignatorByNamespace(gomock.Eq(ctx), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	kafkaInstanceServiceMock.EXPECT().
		GetById(gomock.Eq(ctx), "test").
		Return(&model.KafkaInstance{Id: "first"}, nil).
		AnyTimes()
	trueVal := true

	gomock.InOrder(
		dao.EXPECT().
			Warmup(gomock.Eq(ctx), gomock.Eq("origin"), gomock.Eq("peer")),
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{
				Namespace: "origin",
				Versioned: &trueVal,
			})).
			Return([]*model.TopicRegistration{
				{
					Topic:     "first",
					Instance:  "test",
					Namespace: "origin",
					Classifier: &model.Classifier{
						Name:      "first-topic",
						Namespace: "origin",
					},
					TopicSettings: model.TopicSettings{
						Versioned: true,
					},
					CreateReq: `{
						"name": "{{namespace}}-first-topic",
						"classifier": {
							"namespace": "origin",
							"name": "first-topic"
						},
						"versioned": true
					}`,
				},
				{
					Topic:     "second",
					Instance:  "test",
					Namespace: "origin",
					Classifier: &model.Classifier{
						Name:      "second-topic",
						Namespace: "origin",
					},
					TopicSettings: model.TopicSettings{
						Versioned: true,
					},
					CreateReq: `{
						"name": "{{namespace}}-second-topic",
						"classifier": {
							"namespace": "origin",
							"name": "second-topic"
						},
						"versioned": true
					}`,
				},
			}, nil).
			Times(1),
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Classifier: model.Classifier{
				Name:      "first-topic",
				Namespace: "peer",
			}})).Times(1),
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{
				Topic:     "peer-first-topic",
				Namespace: "peer",
			})).Times(1),
		dao.EXPECT().
			InsertTopicRegistration(gomock.Eq(ctx), gomock.Eq(&expectedFirstTopicReg), gomock.Any()),
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{Classifier: model.Classifier{
				Name:      "second-topic",
				Namespace: "peer",
			}})).Times(1),
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{
				Topic:     "peer-second-topic",
				Namespace: "peer",
			})).Times(1),
		dao.EXPECT().
			InsertTopicRegistration(gomock.Eq(ctx), gomock.Eq(&expectedSecondTopicReg), gomock.Any()),
	)

	err := kafkaService.Warmup(ctx, &domain.BGState{
		ControllerNamespace: "controller",
		Origin: &domain.BGNamespace{
			Name:    "origin",
			State:   "active",
			Version: "v1",
		},
		Peer: &domain.BGNamespace{
			Name:  "peer",
			State: "idle",
		},
		UpdateTime: time.Now(),
	})
	assert.NoError(t, err)
}

func TestKafkaServiceImpl_Commit(t *testing.T) {
	testInitializer(t)

	dao := NewMockKafkaDao(mockCtrl)

	kafkaService := NewKafkaService(dao, kafkaInstanceServiceMock, kafkaHelper, nil, nil, nil, nil)

	expectedFirstTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "first-topic", Namespace: "origin"},
		Topic:         "first",
		Instance:      "test",
		Namespace:     "origin",
		TopicSettings: model.TopicSettings{Versioned: true},
		InstanceRef: &model.KafkaInstance{
			Id: "first",
		},
		CreateReq: `{
						"name": "{{namespace}}-first-topic",
						"classifier": {
							"namespace": "origin",
							"name": "first-topic"
						},
						"versioned": true
					}`,
	}

	expectedSecondTopicReg := model.TopicRegistration{
		Classifier:    &model.Classifier{Name: "second-topic", Namespace: "origin"},
		Topic:         "second",
		Instance:      "test",
		Namespace:     "origin",
		TopicSettings: model.TopicSettings{Versioned: true},
		InstanceRef: &model.KafkaInstance{
			Id: "first",
		},
		CreateReq: `{
						"name": "{{namespace}}-second-topic",
						"classifier": {
							"namespace": "origin",
							"name": "second-topic"
						},
						"versioned": true
					}`,
	}

	kafkaInstanceServiceMock.EXPECT().
		GetById(gomock.Eq(ctx), "test").
		Return(&model.KafkaInstance{Id: "first"}, nil).
		AnyTimes()

	trueVal := true
	gomock.InOrder(
		dao.EXPECT().
			FindTopicsBySearchRequest(gomock.Eq(ctx), gomock.Eq(&model.TopicSearchRequest{
				Namespace: "origin",
				Versioned: &trueVal,
			})).
			Return([]*model.TopicRegistration{
				{
					Topic:     "first",
					Instance:  "test",
					Namespace: "origin",
					Classifier: &model.Classifier{
						Name:      "first-topic",
						Namespace: "origin",
					},
					TopicSettings: model.TopicSettings{
						Versioned: true,
					},
					CreateReq: `{
						"name": "{{namespace}}-first-topic",
						"classifier": {
							"namespace": "origin",
							"name": "first-topic"
						},
						"versioned": true
					}`,
				},
				{
					Topic:     "second",
					Instance:  "test",
					Namespace: "origin",
					Classifier: &model.Classifier{
						Name:      "second-topic",
						Namespace: "origin",
					},
					TopicSettings: model.TopicSettings{
						Versioned: true,
					},
					CreateReq: `{
						"name": "{{namespace}}-second-topic",
						"classifier": {
							"namespace": "origin",
							"name": "second-topic"
						},
						"versioned": true
					}`,
				},
			}, nil).
			Times(1),
		dao.EXPECT().
			DeleteTopicRegistration(gomock.Eq(ctx), gomock.Eq(&expectedFirstTopicReg), gomock.Any()),
		dao.EXPECT().
			DeleteTopicRegistration(gomock.Eq(ctx), gomock.Eq(&expectedSecondTopicReg), gomock.Any()),
	)
	err := kafkaService.Commit(ctx, &domain.BGState{
		ControllerNamespace: "controller",
		Origin: &domain.BGNamespace{
			Name:    "origin",
			State:   "idle",
			Version: "v3",
		},
		Peer: &domain.BGNamespace{
			Name:    "peer",
			State:   "active",
			Version: "v2",
		},
		UpdateTime: time.Now(),
	})

	assert.NoError(t, err)

}
