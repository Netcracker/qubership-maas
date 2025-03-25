package rabbit_service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dao"
	dbc "maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/model"
	"maas/maas-service/monitoring"
	"maas/maas-service/msg"
	mock_auth "maas/maas-service/service/auth/mock"
	"maas/maas-service/service/bg2/domain"
	mock_domain "maas/maas-service/service/bg2/domain/mock"
	"maas/maas-service/service/bg_service"
	mock_bg_service "maas/maas-service/service/bg_service/mock_service"
	instance2 "maas/maas-service/service/instance"
	mock_instance "maas/maas-service/service/instance/mock"
	"maas/maas-service/service/rabbit_service/helper"
	mock_helper "maas/maas-service/service/rabbit_service/helper/mock"
	mock_rabbit_service "maas/maas-service/service/rabbit_service/mock"
	"maas/maas-service/testharness"
	"testing"
	"time"
)

var (
	assertion    *assert.Assertions
	mockCtrl     *gomock.Controller
	ctx          context.Context = nil
	rabbitHelper *mock_helper.MockRabbitHelper
	km           *mock_rabbit_service.MockKeyManager
	auditService *monitoring.MockAuditor

	rabbitInstanceServiceMock *mock_instance.MockRabbitInstanceService

	rabbitHelperMockFunc RabbitHelperFunc

	namespace             = "test-namespace"
	db                    *pg.DB
	daoImpl               dao.BaseDao
	rabbitDao             RabbitServiceDao
	mockRabbitDao         *mock_rabbit_service.MockRabbitServiceDao
	mockRabbitInstanceDao *mock_instance.MockRabbitInstancesDao
	mockBgService         *mock_bg_service.MockBgService
	instanceService       instance2.RabbitInstanceService
	domainService         domain.BGDomainService
	rabbitInstance        *model.RabbitInstance
	rabbitService         RabbitService
	authService           *mock_auth.MockAuthService
	classifier            model.Classifier
	err                   error
	vhost                 *model.VHostRegistration
)

func testInitializer(t *testing.T) {
	assertion = assert.New(t)
	mockCtrl = gomock.NewController(t)
	mockRabbitDao = mock_rabbit_service.NewMockRabbitServiceDao(mockCtrl)
	mockRabbitInstanceDao = mock_instance.NewMockRabbitInstancesDao(mockCtrl)
	mockBgService = mock_bg_service.NewMockBgService(mockCtrl)
	rabbitHelper = mock_helper.NewMockRabbitHelper(mockCtrl)
	km = mock_rabbit_service.NewMockKeyManager(mockCtrl)
	auditService = monitoring.NewMockAuditor(mockCtrl)
	authService = mock_auth.NewMockAuthService(mockCtrl)
	rabbitInstanceServiceMock = mock_instance.NewMockRabbitInstanceService(mockCtrl)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = context.Background()
	ctx = model.WithRequestContext(ctx, requestContext)

	rabbitHelperMockFunc = func(ctx context.Context, s RabbitService, instance *model.RabbitInstance, vhost *model.VHostRegistration, classifier *model.Classifier) (helper.RabbitHelper, error) {
		return rabbitHelper, nil
	}
}

func initDbAndDaoAndDomainAndInstance(tdb *testharness.TestDatabase) {
	requestContext := &model.RequestContext{}
	requestContext.Namespace = namespace
	ctx = model.WithRequestContext(context.Background(), requestContext)

	db = pg.Connect(&pg.Options{
		Addr:     fmt.Sprintf("%s:%d", tdb.Host(), tdb.Port()),
		User:     tdb.Username(),
		Password: tdb.Password(),
		Database: tdb.DBName(),
	})
	daoImpl = dao.New(&dbc.Config{
		Addr:      tdb.Addr(),
		User:      tdb.Username(),
		Password:  tdb.Password(),
		Database:  tdb.DBName(),
		PoolSize:  10,
		DrMode:    dr.Active,
		CipherKey: "thisis32bitlongpassphraseimusing",
	})

	domainService = domain.NewBGDomainService(domain.NewBGDomainDao(daoImpl))

	rabbitDao = NewRabbitServiceDao(daoImpl, domainService.FindByNamespace)

	instanceService = instance2.NewRabbitInstanceServiceWithHealthChecker(
		instance2.NewRabbitInstancesDao(daoImpl),
		func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)

	rabbitInstance = &model.RabbitInstance{
		Id:       "test-instance",
		ApiUrl:   "url",
		AmqpUrl:  "url",
		User:     "user",
		Password: "password",
		Default:  true,
	}

	_, err := instanceService.Register(ctx, rabbitInstance)
	if err != nil {
		assertion.FailNow("Error during Insert of RabbitInstance", err)
		log.ErrorC(ctx, "Error during Insert of RabbitInstance: %v", err)
		return
	}

}

func initRabbitServiceAndVhostCreation() {
	km.EXPECT().
		SecurePassword(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return("km:secured", nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		CreateVHost(gomock.Any()).
		Return(nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		FormatCnnUrl(gomock.Any()).
		Return("").
		AnyTimes()
	auditService.EXPECT().
		AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	rabbitService = NewRabbitServiceWithHelper(rabbitDao, instanceService, km, rabbitHelperMockFunc, auditService, nil, domainService, authService)

	classifier = model.Classifier{
		Name:      "test",
		Namespace: namespace,
	}

	_, vhost, err = rabbitService.GetOrCreateVhost(ctx, rabbitInstance.Id, &classifier, nil)
	assertion.NoError(err)
}

func TestRabbitService_ApplyConfigWithExchange(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	requestEntities := model.RabbitEntities{
		Exchanges: []interface{}{map[string]interface{}{"name": "test-exchange-2", "type": "direct",
			"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
		},
	}

	gomock.InOrder(
		rabbitHelper.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "test-exchange-2", "type": "direct",
				"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, "", nil).
			Times(1),
		mockRabbitDao.EXPECT().
			UpsertNamedRabbitEntity(gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		mockRabbitDao.EXPECT().
			GetNotCreatedLazyBindingsByClassifier(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
	)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = "test-namespace"
	ctx = model.WithRequestContext(context.Background(), requestContext)
	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
	result, update, err := service.CreateOrUpdateEntitiesV1(ctx, &model.VHostRegistration{}, requestEntities)
	assertion.NotNil(result)
	assertion.Nil(update)
	assertion.Nil(err)
}

func TestRegistrationService_CreateAndRegisterVHost(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := &model.RabbitInstance{
		Id:       "",
		ApiUrl:   "",
		AmqpUrl:  "ampq://abc.cde",
		User:     "",
		Password: "",
		Default:  false,
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		km.EXPECT().
			SecurePassword(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
			Return("km:secured", nil).
			Times(1),
		mockRabbitDao.EXPECT().
			InsertVhostRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		rabbitHelper.EXPECT().
			FormatCnnUrl(gomock.Any()).
			Return(fmt.Sprintf("ampq://abc.cde/%s.%s", "ns1", "test")).
			Times(1),
	)
	expected := &model.VHostRegistrationResponse{
		Username: "scott",
		Password: "km:secured",
		Cnn:      fmt.Sprintf("ampq://abc.cde/%s.%s", "ns1", "test"),
	}

	// =======================================================
	// perform test
	// =======================================================
	//service := NewRabbitService(mockRabbitDao, km).(*RabbitServiceImpl)
	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
	if reg, err := service.CreateAndRegisterVHost(ctx, instance, &model.Classifier{Name: "test"}, nil); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		// ignore username
		reg.Username = expected.Username
		assertion.Equal(expected, reg)
	}
}

func TestRegistrationService_VhostInstanceUpdate(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	namespace := "test-namespace"

	instance := model.RabbitInstance{
		Id:      "test-instance",
		Default: false,
	}

	newInstance := model.RabbitInstance{
		Id:      "new-instance",
		Default: false,
	}

	classifier := model.Classifier{
		Name:      "test-name",
		Namespace: namespace,
	}
	classifierStr := classifierToStr(classifier)

	regMock := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		InstanceId: instance.Id,
		Namespace:  namespace,
		Classifier: classifierStr,
	}

	newReg := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		InstanceId: newInstance.Id,
		Namespace:  namespace,
		Classifier: classifierStr,
	}

	gomock.InOrder(

		mockRabbitDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(&regMock, nil).
			Times(1),
		mockRabbitDao.EXPECT().
			DeleteVhostRegistration(eqCtx, gomock.Eq(&regMock)).
			Return(nil).
			Times(1),
		rabbitInstanceServiceMock.EXPECT().
			GetRabbitInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
		rabbitInstanceServiceMock.EXPECT().
			GetById(eqCtx, gomock.Eq(newInstance.Id)).
			Return(&newInstance, nil).
			Times(1),
		km.EXPECT().
			SecurePassword(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
			Return("km:secured", nil).
			Times(1),
		mockRabbitDao.EXPECT().
			InsertVhostRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		rabbitHelper.EXPECT().
			FormatCnnUrl(gomock.Any()).
			Return(fmt.Sprintf("ampq://abc.cde/%s.%s", "ns1", "test")).
			Times(1),

		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(&newReg, nil).
			Times(1),
	)

	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Eq("new-instance")).
		Times(1)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).Times(1)

	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
	if found, reg, err := service.GetOrCreateVhost(ctx, newInstance.Id, &classifier, nil); err != nil {
		t.Fatalf("Unexpected error")
	} else {
		assertion.Equal(false, found)
		assertion.Equal(&newReg, reg)
	}
}

func TestRegistrationService_VhostInstanceUpdate_Default(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	namespace := "test-namespace"

	instance := model.RabbitInstance{
		Id:      "test-instance",
		Default: false,
	}

	classifier := model.Classifier{
		Name:      "test-name",
		Namespace: namespace,
	}
	classifierStr := classifierToStr(classifier)

	regMock := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		InstanceId: instance.Id,
		Namespace:  namespace,
		Classifier: classifierStr,
	}

	newReg := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		InstanceId: instance.Id,
		Namespace:  namespace,
		Classifier: classifierStr,
	}

	gomock.InOrder(
		rabbitInstanceServiceMock.EXPECT().
			GetRabbitInstanceDesignatorByNamespace(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes(),
		mockRabbitDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(&regMock, nil).
			Times(1),
	)
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
	if found, reg, err := service.GetOrCreateVhost(ctx, "", &classifier, nil); err != nil {
		t.Fatalf("Unexpected error")
	} else {
		assertion.Equal(true, found)
		assertion.Equal(&newReg, reg)
	}
}

func TestFindClassifierAndNamespace(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)
	classifier := model.Classifier{
		TenantId: "007",
		Name:     "test-name",
	}
	classifierStr := classifierToStr(classifier)
	namespace := "test-namespace"

	regMock := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		Namespace:  "test-namespace",
		Classifier: classifierStr,
	}
	gomock.InOrder(
		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(&regMock, nil).
			Times(1),
	)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	service := NewRabbitService(mockRabbitDao, rabbitInstanceServiceMock, km, auditService, nil, nil, authService)
	reg, err := service.FindVhostByClassifier(ctx, &classifier)
	if err != nil {
		t.Fatalf("Failed: %v", err.Error())
	}
	assertion.Equal(namespace, reg.Namespace)
	assertion.Equal(classifierStr, reg.Classifier)
}

func TestFindWithSearchFormByNamespace(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)
	classifier := model.Classifier{
		TenantId: "007",
		Name:     "test-name",
	}
	classifierStr := classifierToStr(classifier)
	namespace := "test-namespace"

	searchForm := model.SearchForm{
		Namespace: namespace,
	}

	regMock := model.VHostRegistration{
		Vhost:      "foo",
		User:       "scott",
		Password:   "tiger",
		Namespace:  "test-namespace",
		Classifier: classifierStr,
	}
	gomock.InOrder(
		mockRabbitDao.EXPECT().
			FindVhostWithSearchForm(eqCtx, gomock.Any()).
			Return([]model.VHostRegistration{regMock}, nil).
			Times(1),
	)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	service := NewRabbitService(mockRabbitDao, rabbitInstanceServiceMock, km, auditService, nil, nil, authService)
	reg, err := service.FindVhostWithSearchForm(ctx, &searchForm)
	if err != nil {
		t.Fatalf("Failed: %v", err.Error())
	}
	assertion.Equal(1, len(reg))
}

func TestInstanceRegistrationService_Register(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432",
		User:     "test",
		Password: "test",
	}

	resultInstance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432/api",
		User:     "test",
		Password: "test",
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockRabbitInstanceDao.EXPECT().
			InsertInstanceRegistration(eqCtx, gomock.Eq(&instance)).
			Return(&resultInstance, nil).
			Times(1),
	)
	rabbitInstanceService := instance2.NewRabbitInstanceServiceWithHealthChecker(
		mockRabbitInstanceDao, func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)

	_, err := rabbitInstanceService.Register(ctx, &instance)
	assertion.Equal(err, nil)
	assertion.Equal(instance.ApiUrl, "http://test.com:5432/api")
}

func TestInstanceRegistrationService_Update(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432",
		User:     "test1",
		Password: "test1",
	}

	resultInstance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432/api",
		User:     "test1",
		Password: "test1",
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockRabbitInstanceDao.EXPECT().
			UpdateInstanceRegistration(eqCtx, gomock.Eq(&resultInstance)).
			Return(nil, nil).
			Times(1),
	)

	rabbitInstanceService := instance2.NewRabbitInstanceServiceWithHealthChecker(
		mockRabbitInstanceDao, func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)
	_, err := rabbitInstanceService.Update(ctx, &instance)
	assertion.Equal(err, nil)
}

func TestInstanceRegistrationService_UpdateForInvalidRabbitInstance(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432",
		User:     "test1",
		Password: "test1",
	}

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(

		mockRabbitInstanceDao.EXPECT().
			UpdateInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, errors.New("some rabbitDao error")).
			Times(1),
	)
	rabbitInstanceService := instance2.NewRabbitInstanceServiceWithHealthChecker(
		mockRabbitInstanceDao, func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)
	_, err := rabbitInstanceService.Update(ctx, &instance)
	assertion.Error(err, "some rabbitDao error")
}

func TestInstanceRegistrationService_Unregister(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	instance := model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432",
		User:     "test",
		Password: "test",
	}
	expectedInstance := instance
	expectedInstance.ApiUrl = expectedInstance.ApiUrl + "/api"

	eqCtx := gomock.Eq(ctx)
	gomock.InOrder(
		mockRabbitInstanceDao.EXPECT().
			RemoveInstanceRegistration(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
	)
	rabbitInstanceService := instance2.NewRabbitInstanceServiceWithHealthChecker(
		mockRabbitInstanceDao, func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)
	_, err := rabbitInstanceService.Unregister(ctx, "instance_id")
	assertion.Equal(err, nil)
}

func classifierToStr(classifier model.Classifier) string {
	cs, _ := json.Marshal(&classifier)
	return string(cs)
}

func TestRabbitService_RemoveVHosts_dontRemoveCuzEmptySearchForm(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	rabbitService := NewRabbitService(mockRabbitDao, rabbitInstanceServiceMock, km, auditService, nil, nil, authService)

	searchForm := &model.SearchForm{}

	err := rabbitService.RemoveVHosts(ctx, searchForm, "")

	assertion.ErrorIs(err, msg.BadRequest)
}

func TestRabbitService_CreateNewVhostWithDesignatorSelector(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	rabbitInstanceFromDesig := &model.RabbitInstance{
		Id:       "test",
		ApiUrl:   "http://test.com:5432",
		User:     "test1",
		Password: "test1",
	}

	namespace := "test-namespace"
	tenantId := "test-tenant"

	classifier := model.Classifier{
		Name: "testTopic", Namespace: namespace, TenantId: tenantId,
	}

	expected := &model.VHostRegistration{
		Id:         0,
		Vhost:      "abc",
		User:       "cde",
		Password:   "abc",
		Namespace:  "cde",
		InstanceId: "abc",
		Classifier: "cde",
	}

	defaultInstanceDesig := "default-designator-instance"
	designator := model.InstanceDesignatorRabbit{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: &defaultInstanceDesig,
		InstanceSelectors: []*model.InstanceSelectorRabbit{
			{
				Id:                   1,
				InstanceDesignatorId: 1,
				ClassifierMatch:      model.ClassifierMatch{TenantId: tenantId},
				InstanceId:           rabbitInstanceFromDesig.Id,
				Instance:             rabbitInstanceFromDesig,
			},
		},
		DefaultInstance: nil,
	}

	domainService := mock_domain.NewMockBGDomainService(mockCtrl)

	gomock.InOrder(
		mockRabbitDao.EXPECT().
			WithLock(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(ctx context.Context, lock string, f func(ctx context.Context) error) error {
				return f(ctx)
			}).
			Times(1),
		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(nil, nil).
			Times(1),
		km.EXPECT().
			SecurePassword(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
			Return("km:secured", nil).
			Times(1),
		mockRabbitDao.EXPECT().
			InsertVhostRegistration(eqCtx, gomock.Any(), gomock.Any()).
			Return(nil).
			Times(1),
		rabbitHelper.EXPECT().
			FormatCnnUrl(gomock.Any()).
			Return(fmt.Sprintf("ampq://abc.cde/%s.%s", "ns1", "test")).
			Times(1),

		mockRabbitDao.EXPECT().
			FindVhostByClassifier(eqCtx, gomock.Any()).
			Return(expected, nil).
			Times(1),
	)

	domainService.EXPECT().
		FindByNamespace(gomock.Any(), namespace).
		Return(nil, nil).
		AnyTimes()
	rabbitInstanceServiceMock.EXPECT().
		GetRabbitInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
		Return(&designator, nil).
		AnyTimes()
	auditService.EXPECT().
		AddEntityRequestStat(eqCtx, gomock.Any(), gomock.Any(), gomock.Any()).
		Times(1)
	authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, domainService, authService)
	if found, reg, err := service.GetOrCreateVhost(ctx, "", &classifier, nil); err != nil {
		t.Fatalf("Failed: %v", err.Error())
	} else {
		assertion.False(found)
		assertion.Equal(expected, reg)
	}

}

func TestRabbitService_CreateNewVhostWildcardDesignator(t *testing.T) {
	testInitializer(t)
	defer mockCtrl.Finish()

	eqCtx := gomock.Eq(ctx)

	asteriskInstance := &model.RabbitInstance{
		Id:      "asterisk-instance",
		ApiUrl:  "https://test:15672",
		AmqpUrl: "amqp://test:5672",
	}

	qmInstance := &model.RabbitInstance{
		Id:      "qm-instance",
		ApiUrl:  "https://test:15672",
		AmqpUrl: "amqp://test:5672",
	}

	fullNamedInstance := &model.RabbitInstance{
		Id:      "full-named-instance",
		ApiUrl:  "https://test:15672",
		AmqpUrl: "amqp://test:5672",
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

	designator := model.InstanceDesignatorRabbit{
		Id:                1,
		Namespace:         namespace,
		DefaultInstanceId: nil,
		InstanceSelectors: []*model.InstanceSelectorRabbit{
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

	domainService := mock_domain.NewMockBGDomainService(mockCtrl)

	rabbitInstanceServiceMock.EXPECT().
		GetRabbitInstanceDesignatorByNamespace(eqCtx, gomock.Any()).
		Return(&designator, nil).
		AnyTimes()
	domainService.EXPECT().
		FindByNamespace(gomock.Any(), namespace).
		Return(nil, nil).
		AnyTimes()

	// =======================================================
	// perform test
	// =======================================================

	service := NewRabbitServiceWithHelper(mockRabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, domainService, authService).(*RabbitServiceImpl)

	instance, err := service.resolveRabbitInstance(ctx, asteriskTopicReg.Instance, *asteriskTopicReg.Classifier, namespace)
	assertion.NoError(err)
	assertion.Equal("asterisk-instance", instance.Id)

	instance, err = service.resolveRabbitInstance(ctx, qmTopicReg.Instance, *qmTopicReg.Classifier, namespace)
	assertion.NoError(err)
	assertion.Equal("qm-instance", instance.Id)

	instance, err = service.resolveRabbitInstance(ctx, fullNamedTopicReg.Instance, *fullNamedTopicReg.Classifier, namespace)
	assertion.NoError(err)
	assertion.Equal("full-named-instance", instance.Id)
}

func TestConfigV1StoringDoubleInsertAndUpdateAndDelete(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		requestContext := &model.RequestContext{}
		requestContext.Namespace = namespace
		ctx = model.WithRequestContext(context.Background(), requestContext)

		db = pg.Connect(&pg.Options{
			Addr:     fmt.Sprintf("%s:%d", tdb.Host(), tdb.Port()),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
		})
		daoImpl = dao.New(&dbc.Config{
			Addr:      tdb.Addr(),
			User:      tdb.Username(),
			Password:  tdb.Password(),
			Database:  tdb.DBName(),
			PoolSize:  3,
			DrMode:    dr.Active,
			CipherKey: "thisis32bitlongpassphraseimusing",
		})

		rabbitDao = NewRabbitServiceDao(daoImpl, func(ctx2 context.Context, s string) (*domain.BGNamespaces, error) {
			return nil, nil
		})

		instanceManager := instance2.NewRabbitInstanceServiceWithHealthChecker(
			instance2.NewRabbitInstancesDao(daoImpl),
			func(rabbitInstance *model.RabbitInstance) error {
				return nil
			},
		)
		_, err := instanceManager.Register(ctx, &model.RabbitInstance{
			Id:       "test-instance",
			ApiUrl:   "url",
			AmqpUrl:  "url",
			User:     "user",
			Password: "password",
			Default:  true,
		})
		if err != nil {
			assertion.FailNow("Error during Insert of RabbitInstance", err)
			log.ErrorC(ctx, "Error during Insert of RabbitInstance: %v", err)
			return
		}

		classifier := model.Classifier{
			Name:      "test",
			Namespace: namespace,
		}

		vhost := &model.VHostRegistration{
			Id:         1,
			Vhost:      "1",
			Namespace:  namespace,
			Classifier: classifier.String(),
			InstanceId: "test-instance",
		}

		err = rabbitDao.InsertVhostRegistration(ctx, vhost, func(reg *model.VHostRegistration) error {
			return nil
		})
		if err != nil {
			assertion.FailNow("Error during Insert of Rabbit Vhost", err)
			log.ErrorC(ctx, "Error during Insert of Rabbit Vhost: %v", err)
			return
		}

		testInitializer(t)
		defer mockCtrl.Finish()

		requestEntitiesInsert := model.RabbitEntities{
			Exchanges: []interface{}{map[string]interface{}{"name": "test-exchange", "type": "direct",
				"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
			},
			Queues: []interface{}{map[string]interface{}{"name": "test-queue",
				"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
			},
			Bindings: []interface{}{map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
				"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
				map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}},
		}

		requestEntitiesUpdate := model.RabbitEntities{
			Exchanges: []interface{}{map[string]interface{}{"name": "test-exchange", "type": "direct",
				"durable": "false", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
			},
			Queues: []interface{}{map[string]interface{}{"name": "test-queue",
				"durable": "false", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
			},
			Bindings: []interface{}{map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
				"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
				map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}},
		}

		deleteEntities := model.RabbitDeletions{
			RabbitEntities: model.RabbitEntities{
				Exchanges: []interface{}{map[string]interface{}{"name": "test-exchange", "type": "direct",
					"durable": "false", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
				},
				Queues: []interface{}{map[string]interface{}{"name": "test-queue",
					"durable": "false", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}},
				},
				Bindings: []interface{}{map[string]interface{}{"source": "test-exchange",
					"destination": "test-queue"},
				},
			},
		}

		gomock.InOrder(
			rabbitHelper.EXPECT().
				CreateExchange(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-exchange", "type": "direct",
					"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateQueue(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-queue",
					"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}, nil).
				Times(1),

			rabbitHelper.EXPECT().
				CreateExchange(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-exchange", "type": "direct",
					"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateQueue(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-queue",
					"durable": "true", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}, nil).
				Times(1),

			rabbitHelper.EXPECT().
				CreateExchange(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-exchange", "type": "direct",
					"durable": "false", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value21"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateQueue(gomock.Any(), gomock.Any()).
				Return(&map[string]interface{}{"name": "test-queue",
					"durable": "false", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}, "", nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value1", "arg2": "value2"}}, nil).
				Times(1),
			rabbitHelper.EXPECT().
				CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
				Return(map[string]interface{}{"source": "test-exchange", "destination": "test-queue",
					"routing_key": "key", "arguments": map[string]interface{}{"arg1": "value11", "arg2": "value22"}}, nil).
				Times(1),

			rabbitHelper.EXPECT().
				DeleteExchange(gomock.Any(), gomock.Any()).
				Return(nil, nil).
				Times(1),
			rabbitHelper.EXPECT().
				DeleteQueue(gomock.Any(), gomock.Any()).
				Return(nil, nil).
				Times(1),
			rabbitHelper.EXPECT().
				DeleteBinding(gomock.Any(), gomock.Any()).
				Return(nil, nil).
				Times(1),
		)

		service := NewRabbitServiceWithHelper(rabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
		result, update, err := service.CreateOrUpdateEntitiesV1(ctx, vhost, requestEntitiesInsert)

		assertion.NotNil(result)
		assertion.Nil(update)
		assertion.Nil(err)

		service = NewRabbitServiceWithHelper(rabbitDao, rabbitInstanceServiceMock, km, rabbitHelperMockFunc, auditService, nil, nil, authService)
		result, update, err = service.CreateOrUpdateEntitiesV1(ctx, vhost, requestEntitiesInsert)

		assertion.NotNil(result)
		assertion.Nil(update)
		assertion.Nil(err)

		res, err := db.Exec("SELECT FROM rabbit_entities * WHERE entity_name = 'test-exchange'")
		assertion.Nil(err)
		assertion.Equal(1, res.RowsAffected())

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE entity_name = 'test-queue'")
		assertion.Nil(err)
		assertion.Equal(1, res.RowsAffected())

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE binding_source = 'test-exchange'")
		assertion.Nil(err)
		assertion.Equal(2, res.RowsAffected())

		result, update, err = service.CreateOrUpdateEntitiesV1(ctx, vhost, requestEntitiesUpdate)
		assertion.NotNil(result)
		assertion.Nil(update)
		assertion.Nil(err)

		var rabbitEntity model.RabbitEntity

		err = db.Model(&rabbitEntity).Where("entity_name = 'test-exchange'").Select()
		assertion.Nil(err)
		assertion.Equal("false", rabbitEntity.RabbitEntity["durable"])

		err = db.Model(&rabbitEntity).Where("entity_name = 'test-queue'").Select()
		assertion.Nil(err)
		assertion.Equal("false", rabbitEntity.RabbitEntity["durable"])

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE binding_source = 'test-exchange'")
		assertion.Nil(err)
		assertion.Equal(2, res.RowsAffected())

		_, err = service.DeleteEntities(ctx, classifier, deleteEntities)
		assertion.Nil(err)

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE entity_name = 'test-exchange'")
		assertion.Nil(err)
		assertion.Equal(0, res.RowsAffected())

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE entity_name = 'test-queue'")
		assertion.Nil(err)
		assertion.Equal(0, res.RowsAffected())

		res, err = db.Exec("SELECT FROM rabbit_entities * WHERE binding_source = 'test-exchange'")
		assertion.Nil(err)
		assertion.Equal(0, res.RowsAffected())
	})
}

func TestRabbitBg2(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		defer mockCtrl.Finish()

		km.EXPECT().
			SecurePassword(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return("km:secured", nil).
			AnyTimes()
		km.EXPECT().
			DeletePassword(gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()
		rabbitHelper.EXPECT().
			CreateVHost(gomock.Any()).
			Return(nil).
			AnyTimes()
		rabbitHelper.EXPECT().
			FormatCnnUrl(gomock.Any()).
			Return("").
			AnyTimes()
		auditService.EXPECT().
			AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			AnyTimes()
		rabbitHelper.EXPECT().
			DeleteVHost(gomock.Any()).
			Return(nil).
			AnyTimes()
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		originNamespace := "origin"
		peerNamespace := "peer"
		controllerNamespace := "controller"

		service := NewRabbitServiceWithHelper(rabbitDao, instanceService, km, rabbitHelperMockFunc, auditService, nil, domainService, authService)

		classifier = model.Classifier{
			Name:      "test",
			Namespace: originNamespace,
		}

		found, createdVhost, err := service.GetOrCreateVhost(ctx, "test-instance", &classifier, nil)
		assertion.False(found)
		assertion.NotNil(createdVhost)
		assertion.Equal("maas.origin.test", createdVhost.Vhost)

		err = domainService.Bind(ctx, originNamespace, peerNamespace, controllerNamespace)
		assertion.Nil(err)

		err = service.Warmup(ctx, &domain.BGState{
			Origin: &domain.BGNamespace{
				Name:    originNamespace,
				State:   "active",
				Version: "v1",
			},
			Peer: &domain.BGNamespace{
				Name:    peerNamespace,
				State:   "candidate",
				Version: "v2",
			},
			UpdateTime: time.Now(),
		})
		assertion.Nil(err)

		//check old vhost not changed
		getVhost, err := service.FindVhostByClassifier(ctx, &classifier)
		assertion.NotNil(getVhost)
		assertion.Equal("maas.origin.test", getVhost.Vhost)

		//check new vhost was created with v2
		peerClassifier := model.Classifier{
			Name:      "test",
			Namespace: peerNamespace,
		}

		getVhost, err = service.FindVhostByClassifier(ctx, &peerClassifier)
		assertion.Nil(err)
		assertion.Equal("maas.peer.test-v2", getVhost.Vhost)

		err = service.Commit(ctx, &domain.BGState{
			Origin: &domain.BGNamespace{
				Name:    originNamespace,
				State:   "active",
				Version: "v1",
			},
			Peer: &domain.BGNamespace{
				Name:    peerNamespace,
				State:   "idle",
				Version: "",
			},
			UpdateTime: time.Now(),
		})
		assertion.Nil(err)

		getVhost, err = service.FindVhostByClassifier(ctx, &peerClassifier)
		assertion.Nil(err)
		assertion.Nil(getVhost)
	})
}

func TestRabbitServiceImpl_ApplyBgStatusRabbit(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		rabbitHelper.EXPECT().
			GetAllExchanges(gomock.Any()).
			Return([]interface{}{map[string]interface{}{"name": "test-exch"}}, nil)
		rabbitHelper.EXPECT().
			GetExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "test-exch", "arguments": map[string]interface{}{"alternate-exchange": "test-exch-ae", "blue-green": "true"}}, nil)
		rabbitHelper.EXPECT().
			GetExchange(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"name": "test-exch-ae"}, nil)
		rabbitHelper.EXPECT().
			CreateExchangeBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, "", nil)
		rabbitHelper.EXPECT().
			GetExchangeSourceBindings(gomock.Any(), gomock.Any()).
			Return([]interface{}{map[string]interface{}{"destination": "some-exch"}}, nil)
		rabbitHelper.EXPECT().
			DeleteExchangeBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil)
		rabbitHelper.EXPECT().
			DeleteExchange(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).AnyTimes()
		rabbitHelper.EXPECT().
			DeleteQueue(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).AnyTimes()
		rabbitHelper.EXPECT().
			DeleteBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).AnyTimes()
		authService.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		authService.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		err = rabbitDao.InsertRabbitMsConfig(ctx, &model.MsConfig{
			Id:               1,
			Namespace:        namespace,
			MsName:           "ms",
			VhostID:          1,
			CandidateVersion: "v1",
			ActualVersion:    "v1",
		})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         1,
				MsConfigId: 1,
				EntityType: "exchange",
				EntityName: "exchange",
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         2,
				MsConfigId: 1,
				EntityType: "queue",
				EntityName: "queue",
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:                 3,
				MsConfigId:         1,
				EntityType:         "binding",
				BindingSource:      "exchange",
				BindingDestination: "queue",
				RabbitEntity:       map[string]interface{}{},
			})
		assertion.NoError(err)

		bgStatus := bg_service.BgStatusChangeEvent{
			Namespace: namespace,
			Prev: &model.BgStatus{
				Namespace:  namespace,
				Active:     "v2",
				Legacy:     "v1",
				Candidates: []string{"v3"},
			},
			Current: &model.BgStatus{
				Namespace:  namespace,
				Active:     "v3",
				Legacy:     "v2",
				Candidates: nil,
			},
		}

		err = rabbitService.ApplyBgStatus(ctx, &bgStatus)
		assertion.NoError(err)
	})
}

func TestRabbitServiceImpl_ApplyMsConfigAndVersionedEntitiesToDb_Insert(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		rabbitConfig := &model.RabbitConfigReqDto{
			Kind:   model.Kind{},
			Pragma: nil,
			Spec: model.ApplyRabbitConfigReqDto{
				Classifier: classifier,
				InstanceId: "1",
				VersionedEntities: &model.RabbitEntities{
					Exchanges: []interface{}{map[string]interface{}{
						"name": "e",
					}},
					Queues: []interface{}{map[string]interface{}{
						"name": "q",
					}},
					Bindings: []interface{}{map[string]interface{}{
						"source":      "e",
						"destination": "q",
					}},
				},
			},
		}

		versEntities, err := rabbitService.ApplyMsConfigAndVersionedEntitiesToDb(ctx, "test-service", rabbitConfig, 1, "v1", namespace)

		assertion.NoError(err)
		assertion.Nil(versEntities)
	})
}

func TestRabbitServiceImpl_ApplyMsConfigAndVersionedEntitiesToDb_Update(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		err := rabbitDao.InsertRabbitMsConfig(ctx, &model.MsConfig{
			Id:               1,
			Namespace:        namespace,
			MsName:           "test-service",
			VhostID:          1,
			CandidateVersion: "v1",
			ActualVersion:    "v1",
		})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         1,
				MsConfigId: 1,
				EntityType: "exchange",
				EntityName: "e-old",
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         2,
				MsConfigId: 1,
				EntityType: "queue",
				EntityName: "e-old",
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:                 3,
				MsConfigId:         1,
				EntityType:         "binding",
				BindingSource:      "e-old",
				BindingDestination: "q-old",
				RabbitEntity:       map[string]interface{}{},
			})
		assertion.NoError(err)

		rabbitConfig := &model.RabbitConfigReqDto{
			Kind:   model.Kind{},
			Pragma: nil,
			Spec: model.ApplyRabbitConfigReqDto{
				Classifier: classifier,
				InstanceId: "1",
				VersionedEntities: &model.RabbitEntities{
					Exchanges: []interface{}{map[string]interface{}{
						"name": "e",
					}},
					Queues: []interface{}{map[string]interface{}{
						"name": "q",
					}},
					Bindings: []interface{}{map[string]interface{}{
						"source":      "e",
						"destination": "q",
					}},
				},
			},
		}

		versEntities, err := rabbitService.ApplyMsConfigAndVersionedEntitiesToDb(ctx, "test-service", rabbitConfig, 1, "v1", namespace)

		assertion.NoError(err)
		assertion.NotNil(versEntities)
	})
}

func TestRabbitServiceImpl_RecoverVhostsByNamespace(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()

		defer mockCtrl.Finish()

		mockBgService.EXPECT().
			AddBgStatusUpdateCallback(gomock.Any())
		rabbitHelper.EXPECT().
			CreateVHostAndReturnStatus(gomock.Any()).
			Return(201, nil).
			AnyTimes()
		rabbitHelper.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "exchange"}, "", nil)
		rabbitHelper.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "exchange"}, nil).AnyTimes()
		rabbitHelper.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "queue"}, "", nil)
		mockBgService.EXPECT().
			GetBgStatusByNamespace(gomock.Any(), gomock.Any()).
			Return(&model.BgStatus{
				Active:     "v1",
				Legacy:     "",
				Candidates: nil,
			}, nil)
		rabbitHelper.EXPECT().
			GetExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e", "arguments": map[string]interface{}{"alternate-exchange": "e-ae", "blue-green": "true"}}, nil)
		rabbitHelper.EXPECT().
			GetExchange(gomock.Any(), gomock.Any()).
			//Return(map[string]interface{}{"name": "e-ae"}, nil)
			Return(nil, nil)
		rabbitHelper.EXPECT().
			DeleteExchange(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).AnyTimes()
		rabbitHelper.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e"}, "", nil)
		rabbitHelper.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e-ae"}, "", nil)
		rabbitHelper.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e"}, "", nil)
		rabbitHelper.EXPECT().
			CreateExchangeBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, "", nil)
		rabbitHelper.EXPECT().
			CreateExchangeBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, "", nil)
		rabbitHelper.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "queue"}, "", nil)
		rabbitHelper.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e"}, nil).AnyTimes()

		rabbitServiceWithBg := NewRabbitServiceWithHelper(rabbitDao, instanceService, km, rabbitHelperMockFunc, auditService, mockBgService, domainService, authService)

		err := rabbitDao.UpsertNamedRabbitEntity(ctx,
			&model.RabbitEntity{
				Id:         1,
				VhostId:    1,
				Namespace:  namespace,
				Classifier: classifierToStr(classifier),
				EntityType: "exchange",
				EntityName: sql.NullString{
					String: "exchange",
					Valid:  true,
				},
				ClientEntity: map[string]interface{}{"name": "exchange"},
			})
		assertion.NoError(err)

		err = rabbitDao.UpsertNamedRabbitEntity(ctx,
			&model.RabbitEntity{
				Id:         2,
				VhostId:    1,
				Namespace:  namespace,
				Classifier: classifierToStr(classifier),

				EntityType: "queue",
				EntityName: sql.NullString{
					String: "queue",
					Valid:  true,
				},
				ClientEntity: map[string]interface{}{"name": "queue"},
			})
		assertion.NoError(err)

		err = rabbitDao.InsertLazyBinding(ctx,
			&model.RabbitEntity{
				Id:         3,
				VhostId:    1,
				Namespace:  namespace,
				Classifier: classifierToStr(classifier),

				EntityType: "binding",
				BindingSource: sql.NullString{
					String: "queue",
					Valid:  true,
				},
				BindingDestination: sql.NullString{
					String: "exchange",
					Valid:  true,
				},
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitMsConfig(ctx, &model.MsConfig{
			Id:               1,
			Namespace:        namespace,
			MsName:           "test-service",
			VhostID:          1,
			CandidateVersion: "v1",
			ActualVersion:    "v1",
		})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         1,
				MsConfigId: 1,
				EntityType: "exchange",
				EntityName: "e",
				ClientEntity: map[string]interface{}{
					"name": "e",
				},
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         2,
				MsConfigId: 1,
				EntityType: "queue",
				EntityName: "q",
				ClientEntity: map[string]interface{}{
					"name": "q",
				},
			})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:                 3,
				MsConfigId:         1,
				EntityType:         "binding",
				BindingSource:      "e",
				BindingDestination: "q",
				ClientEntity: map[string]interface{}{
					"source":      "e",
					"destination": "q",
				},
			})
		assertion.NoError(err)

		err = rabbitServiceWithBg.RecoverVhostsByNamespace(ctx, namespace)
		assertion.NoError(err)

	})
}

func TestRabbitServiceImpl_RabbitBgValidation(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		err := rabbitService.RabbitBgValidation(ctx, namespace, "v1")
		assertion.NoError(err)
	})
}

func TestRabbitServiceImpl_ApplyMssInActiveButNotInCandidateForVhost(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		err = rabbitDao.InsertRabbitMsConfig(ctx, &model.MsConfig{
			Id:               2,
			Namespace:        namespace,
			MsName:           "test-service",
			VhostID:          1,
			CandidateVersion: "v1",
			ActualVersion:    "v1",
		})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:         2,
				MsConfigId: 2,
				EntityType: "exchange",
				EntityName: "e",
				ClientEntity: map[string]interface{}{
					"name": "e",
				},
			})
		assertion.NoError(err)

		err = rabbitService.ApplyMssInActiveButNotInCandidateForVhost(ctx, *vhost, "v1", "v2")
		assertion.NoError(err)
	})
}

func TestRabbitServiceImpl_GetConnectionUrl(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		resp, err := rabbitService.GetConnectionUrl(ctx, vhost)
		assertion.NoError(err)
		assertion.NotNil(resp)

		//test default instance
		vhost.InstanceId = ""
		resp, err = rabbitService.GetConnectionUrl(ctx, vhost)
		assertion.NoError(err)
		assertion.NotNil(resp)
	})
}

func TestRabbitServiceImpl_CleanupNamespace(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		rabbitHelper.EXPECT().
			DeleteVHost(gomock.Any()).
			Return(nil)
		km.EXPECT().
			DeletePassword(gomock.Any(), gomock.Any()).
			Return(nil)

		err := rabbitService.CleanupNamespace(ctx, namespace)
		assertion.NoError(err)
	})
}

func TestRabbitServiceImpl_GetLazyBindings(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		err = rabbitDao.InsertRabbitMsConfig(ctx, &model.MsConfig{
			Id:               1,
			Namespace:        namespace,
			MsName:           "test-service",
			VhostID:          1,
			CandidateVersion: "v1",
			ActualVersion:    "v1",
		})
		assertion.NoError(err)

		err = rabbitDao.InsertRabbitVersionedEntity(ctx,
			&model.RabbitVersionedEntity{
				Id:                 3,
				MsConfigId:         1,
				EntityType:         "binding",
				BindingSource:      "e",
				BindingDestination: "q",
				ClientEntity: map[string]interface{}{
					"source":      "e",
					"destination": "q",
				},
			})
		assertion.NoError(err)

		bindings, err := rabbitService.GetLazyBindings(ctx, namespace)
		assertion.NoError(err)
		assertion.NotNil(bindings)
	})
}

func TestRabbitServiceImpl_ApplyPolicies(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		rabbitHelper.EXPECT().
			CreatePolicy(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"name": "policy"}, "", nil)

		policies, err := rabbitService.ApplyPolicies(ctx, classifier, []interface{}{map[string]interface{}{"name": "policy"}})
		assertion.NoError(err)
		assertion.NotNil(policies)
	})
}

func TestRabbitServiceImpl_DestroyDomain_Ok(t *testing.T) {
	testInitializer(t)
	rdao := mock_rabbit_service.NewMockRabbitServiceDao(mockCtrl)
	rs := NewRabbitService(rdao, nil, nil, nil, nil, nil, authService)

	rdao.EXPECT().FindVhostsByNamespace(gomock.Eq(ctx), gomock.Eq("core-dev")).Return(nil, nil).Times(1)
	rdao.EXPECT().FindVhostsByNamespace(gomock.Eq(ctx), gomock.Eq("core-dev-peer")).Return(nil, nil).Times(1)

	err := rs.DestroyDomain(ctx, &domain.BGNamespaces{Origin: "core-dev", Peer: "core-dev-peer"})
	assert.NoError(t, err)
}

func TestRabbitServiceImpl_DestroyDomain_Conflict(t *testing.T) {
	testInitializer(t)
	rdao := mock_rabbit_service.NewMockRabbitServiceDao(mockCtrl)
	rs := NewRabbitService(rdao, nil, nil, nil, nil, nil, authService)

	rdao.EXPECT().FindVhostsByNamespace(gomock.Eq(ctx), gomock.Eq("core-dev")).Return(nil, nil).Times(1)
	rdao.EXPECT().FindVhostsByNamespace(gomock.Eq(ctx), gomock.Eq("core-dev-peer")).Return([]model.VHostRegistration{{}}, nil).Times(1)

	err := rs.DestroyDomain(ctx, &domain.BGNamespaces{Origin: "core-dev", Peer: "core-dev-peer"})
	assert.ErrorIs(t, err, msg.Conflict)
}

func TestRabbitServiceImpl_DestroyDomain_DBError(t *testing.T) {
	testInitializer(t)
	rdao := mock_rabbit_service.NewMockRabbitServiceDao(mockCtrl)
	rs := NewRabbitService(rdao, nil, nil, nil, nil, nil, authService)

	rdao.EXPECT().FindVhostsByNamespace(gomock.Eq(ctx), gomock.Eq("core-dev")).Return(nil, fmt.Errorf("oops")).Times(1)

	err := rs.DestroyDomain(ctx, &domain.BGNamespaces{Origin: "core-dev", Peer: "core-dev-peer"})
	assert.Error(t, err)
}

func TestRegistrationService_RotatePassword(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		testInitializer(t)
		initDbAndDaoAndDomainAndInstance(tdb)
		initRabbitServiceAndVhostCreation()
		defer mockCtrl.Finish()

		domainService = domain.NewBGDomainService(domain.NewBGDomainDao(daoImpl))
		rabbitDao = NewRabbitServiceDao(daoImpl, domainService.FindByNamespace)

		rabbitHelper.EXPECT().
			CreateVHost(gomock.Any()).
			Return(nil).
			AnyTimes()

		rabbitHelper.EXPECT().
			CreateOrUpdateUser(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil).
			AnyTimes()

		service := NewRabbitServiceWithHelper(rabbitDao, instanceService, km, rabbitHelperMockFunc, auditService, nil, nil, authService)

		vhostReg, err := service.FindVhostByClassifier(ctx, &classifier)
		assertion.Nil(err)
		assertion.NotNil(vhostReg)

		oldPass := vhostReg.Password

		searchForm := &model.SearchForm{
			Namespace: "test-namespace",
		}

		vhostRegs, err := service.RotatePasswords(ctx, searchForm)
		assertion.Nil(err)
		assertion.Equal(1, len(vhostRegs))

		vhostReg, err = service.FindVhostByClassifier(ctx, &classifier)
		assertion.Nil(err)
		assertion.NotNil(vhostReg)

		assertion.NotEqual(oldPass, vhostReg.Password)

	})

}
