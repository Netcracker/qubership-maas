package configurator_service

import (
	"context"
	"fmt"
	"github.com/go-pg/pg/v10"
	"github.com/golang/mock/gomock"
	_ "github.com/proullon/ramsql/driver"
	_assert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"maas/maas-service/dao"
	dbc "maas/maas-service/dao/db"
	"maas/maas-service/dr"
	"maas/maas-service/encryption"
	"maas/maas-service/model"
	"maas/maas-service/service/bg2"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/bg_service"
	"maas/maas-service/service/instance"
	"maas/maas-service/service/rabbit_service"
	"maas/maas-service/testharness"
	"reflect"
	"testing"
	"time"
)

// The test Suite fixture
type RabbitBgTestSuite struct {
	suite.Suite

	db            *pg.DB
	dao           dao.BaseDao
	rabbitDao     rabbit_service.RabbitServiceDao
	bgDao         bg_service.BgServiceDao
	configService ConfiguratorService
	bgService     bg_service.BgService

	bgManager     bg2.BgManager
	domainService domain.BGDomainService

	namespace string
	ctx       context.Context
	assert    *_assert.Assertions
	// vhost *model.VHostRegistration
}

// Setup the test suite fixture
func (suite *RabbitBgTestSuite) SetupTest() {
	suite.db = db
	suite.dao = daoImpl
	suite.rabbitDao = rabbitDaoImpl
	suite.domainService = domainServiceImpl
	suite.bgDao = bgDaoImpl
	suite.namespace = namespace
	suite.assert = _assert.New(suite.T())

	requestContext := &model.RequestContext{}
	requestContext.Namespace = suite.namespace
	suite.ctx = model.WithRequestContext(context.Background(), requestContext)

	var sqlStatement string
	var err error
	sqlStatement = "DELETE FROM rabbit_versioned_entities"
	_, err = db.Exec(sqlStatement)
	suite.NoError(err)
	sqlStatement = "DELETE FROM rabbit_ms_configs"
	_, err = db.Exec(sqlStatement)
	suite.NoError(err)
	sqlStatement = "DELETE FROM bg_statuses"
	_, err = db.Exec(sqlStatement)
	suite.NoError(err)

	_, err = db.Model(&model.BgStatus{
		Namespace:  namespace,
		Timestamp:  time.Now(),
		Active:     "v1",
		Legacy:     "",
		Candidates: nil,
	}).Insert()
	suite.NoError(err)

	rabbitInstanceService := instance.NewRabbitInstanceService(instance.NewRabbitInstancesDao(suite.dao))
	rabbitInstanceService.Register(context.Background(), &model.RabbitInstance{
		Id:       "default",
		ApiUrl:   "http://localhost:4567/api",
		AmqpUrl:  "amqp://localhost:4567",
		User:     "admin",
		Password: "admin",
		Default:  true,
	})

	testInitializer(suite.T())

	suite.bgService = bg_service.NewBgService(suite.bgDao)
	suite.configService = NewConfiguratorService(nil, rabbitInstanceService, rabbitServiceMock, nil, nil, suite.bgService, suite.domainService, compositeNamespaceManager)

	rabbitService := rabbit_service.NewRabbitServiceWithHelper(suite.rabbitDao, rabbitInstanceService, km, rabbitHelperMockFunc, auditService, suite.bgService, nil, authServiceMock)

	suite.bgManager = *bg2.NewManager(suite.domainService, rabbitService)

	rabbitServiceMock.EXPECT().
		GetOrCreateVhost(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(a, b, c, d interface{}) (bool, *model.VHostRegistration, error) {
		return rabbitService.GetOrCreateVhost(a.(context.Context), b.(string), c.(*model.Classifier), d.(*model.Version))
	}).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		RabbitBgValidation(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, ns string, candidateVersion string) error {
			return rabbitService.RabbitBgValidation(ctx, ns, candidateVersion)
		}).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		ApplyBgStatus(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, ev *bg_service.BgStatusChangeEvent) error {
			return rabbitService.ApplyBgStatus(ctx, ev)
		}).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		Warmup(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, bgState *domain.BGState) error {
			return rabbitService.Warmup(ctx, bgState)
		}).
		AnyTimes()
	km.EXPECT().
		SecurePassword(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return("password", nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		CreateVHost(gomock.Any()).
		Return(nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		GetAllExchanges(gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		DeleteExchange(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		DeleteQueue(gomock.Any(), gomock.Any()).
		Return(nil, nil).
		AnyTimes()
	rabbitHelper.EXPECT().
		FormatCnnUrl(gomock.Any()).
		Return("").
		AnyTimes()
	rabbitServiceMock.EXPECT().
		GetConnectionUrl(gomock.Any(), gomock.Any()).
		Return("", nil).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		ProcessExportedVhost(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		ApplyMsConfigAndVersionedEntitiesToDb(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(a, b, c, d, e, f interface{}) ([]model.RabbitVersionedEntity, error) {
		return rabbitService.ApplyMsConfigAndVersionedEntitiesToDb(a.(context.Context), b.(string), c.(*model.RabbitConfigReqDto), d.(int), e.(string), f.(string))
	}).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		ApplyMssInActiveButNotInCandidateForVhost(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).DoAndReturn(func(a, b, c, d interface{}) error {
		return rabbitService.ApplyMssInActiveButNotInCandidateForVhost(a.(context.Context), b.(model.VHostRegistration), c.(string), d.(string))
	}).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		DeleteEntitiesByRabbitVersionedEntities(gomock.Any(), gomock.Any()).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		CreateVersionedEntities(gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	rabbitServiceMock.EXPECT().
		FindVhostsByNamespace(gomock.Any(), gomock.Any()).
		DoAndReturn(func(a, b interface{}) ([]model.VHostRegistration, error) {
			return rabbitService.FindVhostsByNamespace(a.(context.Context), b.(string))
		}).
		AnyTimes()
	auditService.EXPECT().
		AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()
	rabbitHelper.EXPECT().GetExchange(gomock.Any(), gomock.Any()).AnyTimes()
	rabbitHelper.EXPECT().CreateExchange(gomock.Any(), gomock.Any()).Return(&map[string]interface{}{}, "", nil).AnyTimes()
	rabbitHelper.EXPECT().CreateQueue(gomock.Any(), gomock.Any()).Return(&map[string]interface{}{}, "", nil).AnyTimes()
	rabbitHelper.EXPECT().CreateBinding(gomock.Any(), gomock.Any()).Return(&map[string]interface{}{}, "", nil).AnyTimes()
	rabbitHelper.EXPECT().CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).Return(map[string]interface{}{}, nil).AnyTimes()
	rabbitHelper.EXPECT().CreateExchangeBinding(gomock.Any(), gomock.Any()).Return(&map[string]interface{}{}, "", nil).AnyTimes()

	kafkaServiceMock.EXPECT().Warmup(gomock.Any(), gomock.Any()).AnyTimes()

	authServiceMock.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authServiceMock.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

}

// Run tests in the suite
func TestRabbitBgTestSuite(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

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

		log.InfoC(ctx, "DB addr: %v", tdb.Addr())

		bgDaoImpl = bg_service.NewBgServiceDao(daoImpl)
		domainServiceImpl = domain.NewBGDomainService(domain.NewBGDomainDao(daoImpl))
		rabbitDaoImpl = rabbit_service.NewRabbitServiceDao(daoImpl, domainServiceImpl.FindByNamespace)

		encryptedPassword, err := encryption.NewAes("thisis32bitlongpassphraseimusing").Encrypt("password")
		_assert.NoError(t, err)
		_, err = db.Model(&model.RabbitInstance{
			Id:       "test-instance",
			ApiUrl:   "url",
			AmqpUrl:  "url",
			User:     "user",
			Password: encryptedPassword,
			Default:  true,
		}).OnConflict("DO NOTHING").Insert()

		if err != nil {
			assertion.FailNow("Error during Insert of RabbitInstance", err)
			log.ErrorC(ctx, "Error during Insert of RabbitInstance: %v", err)
			return
		}

		suite.Run(t, new(RabbitBgTestSuite))
	})
}

// This suite's single test
func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_1() {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitesOfMs,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_2() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	//since lazy binding feature it is possible to have binding without exchange
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
	//suite.assert.Contains(err.Error(), "bindings have non-existing exchange source within vhost")
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_3() {
	assert := _assert.New(suite.T())

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        pragma:
            kube-secret: <secret-name>
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`
	configService := NewConfiguratorService(nil, nil, rabbitServiceMock, nil, nil, suite.bgService, nil, compositeNamespaceManager)
	_, err := configService.ApplyConfigV2(suite.ctx, cfg)

	assert.NoError(err)

	actualMsConfigs, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(context.Background(), suite.namespace, "v1")
	assert.NoError(err)
	assert.NotNil(actualMsConfigs)

	vhost, err := suite.rabbitDao.FindVhostByClassifier(ctx, model.Classifier{Name: "vers-test", Namespace: "test-namespace"})
	assert.NoError(err)

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity

	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_4() {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`
	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitesOfMs,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	//since lazy binding feature it is possible to have binding without exchange
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
	//suite.assert.Contains(err.Error(), "bindings have non-existing exchange source within vhost")
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_5() {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`
	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitesOfMs,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg2 := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, actualMsConfigs = applyConfig(suite, cfg2, "v2")
	suite.assert.NotNil(actualMsConfigs)

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	expectedMsConfigs = nil
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_6() {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`
	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitesOfMs,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg2 := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, actualMsConfigs = applyConfig(suite, cfg2, "v2")
	suite.assert.NotNil(actualMsConfigs)

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity

	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	expectedMsConfigs = nil
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_7() {

	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg2 := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
    - serviceName: order-executor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg2, "v2")

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_8() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitesOfMs,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_9() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	//entities to compare with
	var expectedMsConfigs []model.MsConfig
	var entitesOfMs []*model.RabbitVersionedEntity

	entitesOfMs = append(entitesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})

	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitesOfMs,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_10() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	//since lazy binding feature it is possible to have binding without exchange
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
	//suite.assert.Contains(err.Error(), "bindings have non-existing exchange source within vhost")
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_11() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_12() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_13() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
`

	//since lazy binding feature it is possible to have binding without exchange
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
	//suite.assert.Contains(err.Error(), "bindings have non-existing exchange source within vhost")
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_14() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_15() {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: at-least-one-changed-service
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "at-least-one-changed-service",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_16() {
	apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var entitiesOfMs1 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_17() {
	apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_separated_18() {
	apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	err := suite.bgService.ApplyBgStatus(ctx, namespace, &model.BgStatus{
		Namespace:  namespace,
		Timestamp:  time.Now(),
		Active:     "v3",
		Legacy:     "v2",
		Candidates: nil,
	})
	suite.assert.NoError(err)

	mss, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(ctx, suite.namespace, "v1")
	suite.assert.NoError(err)
	suite.assert.Nil(mss)
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_19_20() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))

}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_21() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))

}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_22() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_23() {

	apply_Ms1WithE1AndQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         nil,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v2",
		Entities:         nil,
	})
	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_24() {

	apply_Ms1WithE1AndQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_single_25() {

	apply_Ms1WithE1AndQ1AndBindingOfNotActualVersion(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs := applyConfig(suite, cfg, "v3")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_two_bindings_26() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key1
                - source: e1
                  destination: q1
                  routing_key: key2
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_two_bindings_27() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key1
                - source: e1
                  destination: q1
                  routing_key: key2
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v3
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v3")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})

	var expectedMsConfigs3 []model.MsConfig
	expectedMsConfigs3 = append(expectedMsConfigs3, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs3 = append(expectedMsConfigs3, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v3",
		ActualVersion:    "v3",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs3, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_to_new_E_28() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
                - source: e2
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
`
	vhost, actualMsConfigs2 := applyConfig(suite, cfg, "v2")

	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e2", "destination": "q1", "routing_key": "key"},
	})

	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs3,
	})

	actualMsConfigs1, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(suite.ctx, suite.namespace, "v1")
	suite.assert.NoError(err)

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs1))
	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs2))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_to_new_E_29() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e2
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
`
	vhost, actualMsConfigs2 := applyConfig(suite, cfg, "v2")

	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e2", "destination": "q1", "routing_key": "key"},
	})

	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs3,
	})

	actualMsConfigs1, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(suite.ctx, suite.namespace, "v1")
	suite.assert.NoError(err)

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs1))
	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs2))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_to_new_E_30() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
                - source: e2
                  destination: q1
                  routing_key: key
    - serviceName: order-smth
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
`
	vhost, actualMsConfigs2 := applyConfig(suite, cfg, "v2")

	entitiesOfMs1 = nil
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})

	entitiesOfMs2 = nil
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e2", "destination": "q1", "routing_key": "key"},
	})

	entitiesOfMs3 = nil
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs3,
	})

	actualMsConfigs1, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(suite.ctx, suite.namespace, "v1")
	suite.assert.NoError(err)

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs1))
	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs2))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_or_E_moved_31() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs1 = nil
	entitiesOfMs2 = nil
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	expectedMsConfigs = nil
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_or_E_moved_32() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "non-unique names within vhost")
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_or_E_moved_33() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs1 = nil
	entitiesOfMs2 = nil
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	expectedMsConfigs = nil
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_Q_or_E_moved_34() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "non-unique names within vhost")

}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_update_35() {

	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-smth
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
                  type: direct
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_update_36() {

	apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE3(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: fanout
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: true
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key2
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "fanout"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": true},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key2"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_update_37() {

	apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE3(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config:
    - serviceName: order-executor
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         nil,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         nil,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_update_38() {

	apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE3(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                - name: e3
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                - name: q2
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
                - source: e3
                  destination: q2
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e3",
		ClientEntity: map[string]interface{}{"name": "e3"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q2",
		ClientEntity: map[string]interface{}{"name": "q2"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e3", "destination": "q2", "routing_key": "key"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func (suite *RabbitBgTestSuite) TestConfigV2RabbitBg_update_39() {
	log.InfoC(ctx, "Start of TestConfigV2RabbitBg_update_39")

	apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE3(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	_, err := db.Model(&model.BgStatus{
		Namespace:  namespace,
		Timestamp:  time.Now(),
		Active:     "v2",
		Legacy:     "v1",
		Candidates: nil,
	}).Insert()
	suite.NoError(err)

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
                - name: e3
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs1 = nil
	entitiesOfMs2 = nil
	entitiesOfMs3 = nil
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e3",
		ClientEntity: map[string]interface{}{"name": "e3"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	expectedMsConfigs = nil
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func apply_Ms1WithE1_Ms2WithQ1AndBinding(suite *RabbitBgTestSuite) {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func apply_Ms1WithE1_Ms2WithQ1AndBinding_Ms3WithE3(suite *RabbitBgTestSuite) {
	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-smth
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
                  type: direct
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs1, entitiesOfMs2, entitiesOfMs3 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})
	entitiesOfMs3 = append(entitiesOfMs3, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e2",
		ClientEntity: map[string]interface{}{"name": "e2", "type": "direct"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-smth",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs3,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))
}

func apply_Ms1WithE1_Ms2WithQ1AndBindingOfNotActualVersion(suite *RabbitBgTestSuite) {
	apply_Ms1WithE1_Ms2WithQ1AndBinding(suite)

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                  type: direct
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v2")

	var entitiesOfMs1, entitiesOfMs2 []*model.RabbitVersionedEntity
	entitiesOfMs1 = append(entitiesOfMs1, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1", "type": "direct"},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1", "durable": false},
	})
	entitiesOfMs2 = append(entitiesOfMs2, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         entitiesOfMs1,
	})
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-executor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs2,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	_, err := db.Model(&model.BgStatus{
		Namespace:  namespace,
		Timestamp:  time.Now(),
		Active:     "v2",
		Legacy:     "v1",
		Candidates: nil,
	}).Insert()
	suite.NoError(err)
}

func apply_Ms1WithE1AndQ1AndBindingOfNotActualVersion(suite *RabbitBgTestSuite) {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	vhost, actualMsConfigs := applyConfig(suite, cfg, "v1")

	var entitiesOfMs []*model.RabbitVersionedEntity
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs []model.MsConfig
	expectedMsConfigs = append(expectedMsConfigs, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v1",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs, actualMsConfigs))

	cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v2
  namespace: test-namespace
  services:
    - serviceName: some-ms
      config:
`
	vhost, actualMsConfigs = applyConfig(suite, cfg, "v2")

	entitiesOfMs = nil
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "exchange",
		EntityName:   "e1",
		ClientEntity: map[string]interface{}{"name": "e1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "queue",
		EntityName:   "q1",
		ClientEntity: map[string]interface{}{"name": "q1"},
	})
	entitiesOfMs = append(entitiesOfMs, &model.RabbitVersionedEntity{
		EntityType:   "binding",
		ClientEntity: map[string]interface{}{"source": "e1", "destination": "q1", "routing_key": "key"},
	})

	var expectedMsConfigs2 []model.MsConfig
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "order-processor",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v1",
		Entities:         entitiesOfMs,
	})
	expectedMsConfigs2 = append(expectedMsConfigs2, model.MsConfig{
		Namespace:        suite.namespace,
		MsName:           "some-ms",
		VhostID:          vhost.Id,
		CandidateVersion: "v2",
		ActualVersion:    "v2",
		Entities:         nil,
	})

	suite.assert.True(equalMsSlices(expectedMsConfigs2, actualMsConfigs))

	_, err := db.Model(&model.BgStatus{
		Namespace:  namespace,
		Timestamp:  time.Now(),
		Active:     "v2",
		Legacy:     "v1",
		Candidates: nil,
	}).Insert()
	suite.NoError(err)
}

func applyConfig(suite *RabbitBgTestSuite, cfg string, version string) (*model.VHostRegistration, []model.MsConfig) {
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)

	vhost, err := suite.rabbitDao.FindVhostByClassifier(suite.ctx, model.Classifier{Name: "vers-test", Namespace: "test-namespace"})
	suite.assert.NoError(err)

	actualMsConfigs, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(suite.ctx, suite.namespace, version)
	suite.assert.NoError(err)
	suite.assert.NotNil(actualMsConfigs)

	return vhost, actualMsConfigs
}

func applyConfigWithNamespace(suite *RabbitBgTestSuite, cfg string, version string, namespace string) (*model.VHostRegistration, []model.MsConfig) {

	requestContext := &model.RequestContext{}
	requestContext.Namespace = namespace

	newCtx := model.WithRequestContext(ctx, requestContext)

	_, err := suite.configService.ApplyConfigV2(newCtx, cfg)
	suite.assert.NoError(err)

	vhost, err := suite.rabbitDao.FindVhostByClassifier(suite.ctx, model.Classifier{Name: "vers-test", Namespace: "test-namespace"})
	suite.assert.NoError(err)

	actualMsConfigs, err := suite.rabbitDao.GetMsConfigsByBgDomainAndCandidateVersion(suite.ctx, suite.namespace, version)
	suite.assert.NoError(err)
	suite.assert.NotNil(actualMsConfigs)

	return vhost, actualMsConfigs
}

// validation tests
func (suite *RabbitBgTestSuite) TestValidationEmptyExchangeName() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name:
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "'name' field should be specified for entity")
}

func (suite *RabbitBgTestSuite) TestValidationEmptyQueueName() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name:
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "'name' field should be specified for entity")
}

func (suite *RabbitBgTestSuite) TestValidationExchangeTwoTimesName() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these exchanges or queues have non-unique names within vhost")
}

func (suite *RabbitBgTestSuite) TestValidationQueueTwoTimesName() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                queues:
                - name: q1
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these exchanges or queues have non-unique names within vhost")
}

func (suite *RabbitBgTestSuite) TestValidationExchangeAndQueueCheckedIndependently() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: q1
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
}

func (suite *RabbitBgTestSuite) TestValidationQueueWithoutBinding() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                - name: q2
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these queues have no bindings within vhost")
}

func (suite *RabbitBgTestSuite) TestValidationQueueWithoutBindingButAnotherVhost() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                - name: q2
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test-executor
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q2
                bindings:
                - source: e1
                  destination: q2
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these queues have no bindings within vhost")

}

func (suite *RabbitBgTestSuite) TestValidationBindingWithNonExistingESource() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e2
                  destination: q1
                  routing_key: key
`

	//since lazy binding feature it is possible to have binding without exchange
	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.NoError(err)
	//suite.assert.Contains(err.Error(), "validation error - these bindings have non-existing exchange source within vhost")
}

func (suite *RabbitBgTestSuite) TestValidationBindingWithNonExistingQDestination() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
                - source: e1
                  destination: q2
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these bindings have non-existing queue destination within vhost")

}

func (suite *RabbitBgTestSuite) TestValidationBindingWithNonExistingQDestinationButAnotherVhost() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
                - source: e1
                  destination: q2
                  routing_key: key
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test-executor
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e3
                queues:
                - name: q2
                bindings:
                - source: e3
                  destination: q2
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these bindings have non-existing queue destination within vhost")

}

func (suite *RabbitBgTestSuite) TestValidationBindingDeclaredNotInSameMsAsQueue() {

	cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: test-namespace
  services:
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e1
                queues:
                - name: q1
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: vers-test
              namespace: test-namespace
            versionedEntities:
                exchanges:
                - name: e2
                queues:
                - name: q2
                bindings:
                - source: e2
                  destination: q2
                  routing_key: key
                - source: e1
                  destination: q1
                  routing_key: key
`

	_, err := suite.configService.ApplyConfigV2(suite.ctx, cfg)
	suite.assert.Error(err)
	suite.assert.Contains(err.Error(), "validation error - these bindings are declared not in the same microservice config as their destination queue within vhost")

}

// bg_statuses tests
func (suite *RabbitBgTestSuite) TestCpInstallPromoteRollback() {

	//install candidate
	cpMessageDto := model.CpMessageDto{
		model.CpDeploymentVersion{
			Version:     "v1",
			Stage:       "ACTIVE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
		model.CpDeploymentVersion{
			Version:     "v2",
			Stage:       "CANDIDATE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
	}

	cpMessage, err := cpMessageDto.ConvertToBgStatus()
	assertion.NoError(err)

	err = suite.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	assertion.NoError(err)

	bgStatus, err := suite.bgService.GetBgStatusByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	assertion.Equal("v1", bgStatus.Active)
	assertion.Equal("v2", bgStatus.Candidates[0])

	//promote
	cpMessageDto = model.CpMessageDto{
		model.CpDeploymentVersion{
			Version:     "v1",
			Stage:       "LEGACY",
			CreatedWhen: "2021-08-18T14:43:26.072479Z",
			UpdatedWhen: "2021-08-18T14:43:26.072479Z",
		},
		model.CpDeploymentVersion{
			Version:     "v2",
			Stage:       "ACTIVE",
			CreatedWhen: "2021-08-18T14:43:26.072479Z",
			UpdatedWhen: "2021-08-18T14:43:26.072479Z",
		},
	}

	cpMessage, err = cpMessageDto.ConvertToBgStatus()
	assertion.NoError(err)

	err = suite.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	assertion.NoError(err)

	bgStatus, err = suite.bgService.GetBgStatusByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	assertion.Equal("v2", bgStatus.Active)
	assertion.Equal("v1", bgStatus.Legacy)
	assertion.Nil(bgStatus.Candidates)

	//rollback
	cpMessageDto = model.CpMessageDto{
		model.CpDeploymentVersion{
			Version:     "v1",
			Stage:       "ACTIVE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
		model.CpDeploymentVersion{
			Version:     "v2",
			Stage:       "CANDIDATE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
	}

	cpMessage, err = cpMessageDto.ConvertToBgStatus()
	assertion.NoError(err)

	err = suite.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	assertion.NoError(err)

	bgStatus, err = suite.bgService.GetBgStatusByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	assertion.Equal("v1", bgStatus.Active)
	assertion.Equal("v2", bgStatus.Candidates[0])
}

func (suite *RabbitBgTestSuite) TestCpInstallDeleteCandidate() {

	//install candidate
	cpMessageDto := model.CpMessageDto{
		model.CpDeploymentVersion{
			Version:     "v1",
			Stage:       "ACTIVE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
		model.CpDeploymentVersion{
			Version:     "v3",
			Stage:       "CANDIDATE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
	}

	cpMessage, err := cpMessageDto.ConvertToBgStatus()
	assertion.NoError(err)

	err = suite.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	assertion.NoError(err)

	bgStatus, err := suite.bgService.GetBgStatusByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	assertion.Equal("v1", bgStatus.Active)
	assertion.Equal("v3", bgStatus.Candidates[0])

	cpMessageDto = model.CpMessageDto{
		model.CpDeploymentVersion{
			Version:     "v1",
			Stage:       "ACTIVE",
			CreatedWhen: "2021-08-18T14:42:26.072479Z",
			UpdatedWhen: "2021-08-18T14:42:26.072479Z",
		},
	}

	cpMessage, err = cpMessageDto.ConvertToBgStatus()
	assertion.NoError(err)

	err = suite.bgService.ApplyBgStatus(ctx, namespace, cpMessage)
	assertion.NoError(err)

	bgStatus, err = suite.bgService.GetBgStatusByNamespace(ctx, model.RequestContextOf(ctx).Namespace)
	assertion.Equal("v1", bgStatus.Active)
	assertion.Nil(bgStatus.Candidates)
}

//equals funcs

func equalMsSlices(expected, real []model.MsConfig) bool {
	var count int
	if len(expected) != len(real) {
		return false
	}
	for _, realMs := range real {
		for _, expectedMs := range expected {
			if equalMss(expectedMs, realMs) {
				count++
			}
		}
	}
	if count != len(expected) {
		log.ErrorC(ctx, "Slices are not equal, expected '%v', real '%v'", expected, real)
		return false
	}
	return true
}

func equalMss(expected, real model.MsConfig) bool {
	//without id, vhost, config
	equal := true
	equal = expected.VhostID == real.VhostID && equal
	equal = expected.ActualVersion == real.ActualVersion && equal
	equal = expected.CandidateVersion == real.CandidateVersion && equal
	equal = expected.MsName == real.MsName && equal
	equal = expected.Namespace == real.Namespace && equal
	if !equal {
		return false
	}
	equal = equalEntSlices(expected.Entities, real.Entities) && equal
	return equal
}

func equalEntSlices(expected, real []*model.RabbitVersionedEntity) bool {
	var count int
	if len(expected) != len(real) {
		return false
	}
	for _, realMs := range real {
		for _, expectedMs := range expected {
			if equalEnt(expectedMs, realMs) {
				count++
			}
		}
	}
	if count != len(expected) {
		return false
	}
	return true
}

func equalEnt(expected, real *model.RabbitVersionedEntity) bool {
	//without id, msconfigId, msconfig
	equal := true
	equal = expected.EntityName == real.EntityName && equal
	equal = expected.EntityType == real.EntityType && equal
	equal = reflect.DeepEqual(expected.ClientEntity, real.ClientEntity) && equal
	return equal
}
