package configurator_service

import (
	"context"
	"errors"
	"github.com/golang/mock/gomock"
	"github.com/jucardi/go-streams/v2/streams"
	"github.com/netcracker/qubership-maas/dao"
	dbc "github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/monitoring"
	mock_auth "github.com/netcracker/qubership-maas/service/auth/mock"
	"github.com/netcracker/qubership-maas/service/bg2"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/bg_service"
	instance2 "github.com/netcracker/qubership-maas/service/instance"
	"github.com/netcracker/qubership-maas/service/rabbit_service"
	"github.com/netcracker/qubership-maas/service/rabbit_service/helper"
	mock_rabbit_helper "github.com/netcracker/qubership-maas/service/rabbit_service/helper/mock"
	mock_rabbit_service "github.com/netcracker/qubership-maas/service/rabbit_service/mock"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/netcracker/qubership-maas/utils"
	_ "github.com/proullon/ramsql/driver"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

var (
	configServiceBg2       ConfiguratorService
	configWithRabbit       ConfiguratorService
	rabbitServiceBg2       rabbit_service.RabbitService
	rabbitWithHelper       rabbit_service.RabbitService
	rabbitServiceDaoBg2    rabbit_service.RabbitServiceDao
	bgManagerBg2           bg2.BgManager
	bgManagerBg2WithHelper bg2.BgManager
	authServiceMockBg2     *mock_auth.MockAuthService

	rabbitHelperMockBg2 *mock_rabbit_helper.MockRabbitHelper

	originNamespace = "origin"
	peerNamespace   = "peer"

	originCtx context.Context
	peerCtx   context.Context
)

type Entity map[string]interface{}
type Binding struct {
	Source      string
	Destination string
}

func testInit(t *testing.T, tdb *testharness.TestDatabase, rabbitTest *testharness.TestRabbit) {

	assertion = assert.New(t)
	ctx = context.Background()

	mockCtrlBg2 := gomock.NewController(t)
	rabbitHelperMockBg2 = mock_rabbit_helper.NewMockRabbitHelper(mockCtrlBg2)
	kmMockBg2 := mock_rabbit_service.NewMockKeyManager(mockCtrlBg2)
	auditServiceBg2 := monitoring.NewMockAuditor(mockCtrlBg2)

	requestContext := &model.RequestContext{}
	requestContext.Namespace = originNamespace
	originCtx = model.WithRequestContext(ctx, requestContext)

	requestContext = &model.RequestContext{}
	requestContext.Namespace = peerNamespace
	peerCtx = model.WithRequestContext(ctx, requestContext)

	daoImplBg2 := dao.New(&dbc.Config{
		Addr:      tdb.Addr(),
		User:      tdb.Username(),
		Password:  tdb.Password(),
		Database:  tdb.DBName(),
		PoolSize:  3,
		DrMode:    dr.Active,
		CipherKey: "thisis32bitlongpassphraseimusing",
	})

	log.InfoC(ctx, "DB addr: %v", tdb.Addr())

	domainServiceImplBg2 := domain.NewBGDomainService(domain.NewBGDomainDao(daoImplBg2))
	rabbitServiceDaoBg2 = rabbit_service.NewRabbitServiceDao(daoImplBg2, domainServiceImplBg2.FindByNamespace)

	rabbitHelperMockFuncBg2 := func(ctx context.Context, s rabbit_service.RabbitService, instance *model.RabbitInstance, vhost *model.VHostRegistration, classifier *model.Classifier) (helper.RabbitHelper, error) {
		return rabbitHelperMockBg2, nil
	}

	instanceServiceBg2 := instance2.NewRabbitInstanceServiceWithHealthChecker(
		instance2.NewRabbitInstancesDao(daoImplBg2),
		func(rabbitInstance *model.RabbitInstance) error {
			return nil
		},
	)

	authServiceMockBg2 = mock_auth.NewMockAuthService(mockCtrlBg2)
	bgServiceBg2 := bg_service.NewBgService(bg_service.NewBgServiceDao(daoImplBg2))

	rabbitServiceBg2 = rabbit_service.NewRabbitServiceWithHelper(rabbitServiceDaoBg2, instanceServiceBg2, kmMockBg2, rabbitHelperMockFuncBg2, auditServiceBg2, bgServiceBg2, domainServiceImplBg2, authServiceMockBg2)

	configServiceBg2 = NewConfiguratorService(nil, instanceServiceBg2, rabbitServiceBg2, nil, nil, bgServiceBg2, domainServiceImplBg2, nil)
	bgManagerBg2 = *bg2.NewManager(domainServiceImplBg2, rabbitServiceBg2)

	rabbitWithHelper = rabbit_service.NewRabbitService(rabbitServiceDaoBg2, instanceServiceBg2, kmMockBg2, auditServiceBg2, bgServiceBg2, domainServiceImplBg2, authServiceMockBg2)
	configWithRabbit = NewConfiguratorService(nil, instanceServiceBg2, rabbitWithHelper, nil, nil, bgServiceBg2, domainServiceImplBg2, nil)
	bgManagerBg2WithHelper = *bg2.NewManager(domainServiceImplBg2, rabbitWithHelper)

	rabbitInstance := &model.RabbitInstance{
		Id:       "test-instance",
		ApiUrl:   "url",
		AmqpUrl:  "url",
		User:     "user",
		Password: "password",
		Default:  true,
	}
	_, err := instanceServiceBg2.Register(ctx, rabbitInstance)
	if err != nil {
		assertion.FailNow("Error during Insert of RabbitInstance", err)
		log.ErrorC(ctx, "Error during Insert of RabbitInstance: %v", err)
		return
	}

	if rabbitTest != nil {
		rabbitInstance := &model.RabbitInstance{
			Id:       "test-instance-real",
			ApiUrl:   rabbitTest.ApiUrl(),
			AmqpUrl:  rabbitTest.AmqpUrl(),
			User:     "guest",
			Password: "guest",
			Default:  true,
		}

		_, err := instanceServiceBg2.Register(ctx, rabbitInstance)
		if err != nil {
			assertion.FailNow("Error during Insert of RabbitInstance", err)
			log.ErrorC(ctx, "Error during Insert of RabbitInstance: %v", err)
			return
		}
	}

	kmMockBg2.EXPECT().
		SecurePassword(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return("km:secured", nil).
		AnyTimes()
	rabbitHelperMockBg2.EXPECT().
		CreateVHost(gomock.Any()).
		Return(nil).
		AnyTimes()
	rabbitHelperMockBg2.EXPECT().
		FormatCnnUrl(gomock.Any()).
		Return("").
		AnyTimes()
	auditServiceBg2.EXPECT().
		AddEntityRequestStat(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		AnyTimes()

	kmMockBg2.EXPECT().
		DeletePassword(gomock.Any(), gomock.Any()).
		Return(nil).
		AnyTimes()

	authServiceMockBg2.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	authServiceMockBg2.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
}

func TestConfigV2RabbitBg2_RabbitBg2(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		testInit(t, tdb, nil)

		authServiceMockBg2.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		authServiceMockBg2.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		//initial config
		rabbitHelperMockBg2.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "q1"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e1"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e1"}, nil)

		//clone config
		rabbitHelperMockBg2.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e1"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "q1"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e1"}, nil)

		//update candidate
		rabbitHelperMockBg2.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "q2"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e2"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e2"}, nil)

		//commit
		rabbitHelperMockBg2.EXPECT().
			DeleteVHost(gomock.Any()).
			Return(nil)

		//new warmup
		rabbitHelperMockBg2.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e1"}, "", nil).Times(2)
		rabbitHelperMockBg2.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "q1"}, "", nil).Times(2)
		rabbitHelperMockBg2.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e1"}, nil).Times(2)

		//todo remove when bg1 not used
		rabbitHelperMockBg2.EXPECT().
			GetAllExchanges(gomock.Any()).
			Return(nil, nil).
			AnyTimes()

		//binding will be created first, so it's also test for lazy binind

		cfg := `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: origin
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: origin
            entities:
                queues:
                - name: q1
                  durable: false
                bindings:
                - source: e1
                  destination: q1
                  routing_key: key
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: origin
            entities:
                exchanges:
                - name: e1
                  type: direct
`

		_, err := configServiceBg2.ApplyConfigV2(originCtx, cfg)
		assertion.NoError(err)

		classifierOrigin := model.Classifier{
			Name:      "test",
			Namespace: originNamespace,
		}

		vhost, err := rabbitServiceBg2.FindVhostByClassifier(originCtx, &classifierOrigin)
		assertion.NotNil(vhost)
		assertion.Equal("maas.origin.test", vhost.Vhost)

		bgState := domain.BGState{
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
		}

		err = bgManagerBg2.InitDomain(originCtx, bgState)
		assertion.NoError(err)

		err = bgManagerBg2.Warmup(originCtx, bgState)
		assertion.NoError(err)

		// check copy of vhost

		classifierPeer := model.Classifier{
			Name:      "test",
			Namespace: peerNamespace,
		}

		vhostPeer, err := rabbitServiceBg2.FindVhostByClassifier(peerCtx, &classifierPeer)
		assertion.NotNil(vhostPeer)
		assertion.Equal("maas.peer.test-v2", vhostPeer.Vhost)

		//update candidate

		cfg = `
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: peer
  services:
    - serviceName: order-executor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: peer
            entities:
                queues:
                - name: q2
                  durable: false
                bindings:
                - source: e2
                  destination: q2
                  routing_key: key
    - serviceName: order-processor
      config: |+
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: peer
            entities:
                exchanges:
                - name: e2
                  type: direct
`

		_, err = configServiceBg2.ApplyConfigV2(peerCtx, cfg)
		assertion.NoError(err)

		entities, err := rabbitServiceDaoBg2.GetRabbitEntitiesByVhostId(peerCtx, vhostPeer.Id)
		assertion.NoError(err)
		assertion.Equal(6, len(entities))

		bgState = domain.BGState{
			Origin: &domain.BGNamespace{
				Name:    originNamespace,
				State:   "idle",
				Version: "",
			},
			Peer: &domain.BGNamespace{
				Name:    peerNamespace,
				State:   "active",
				Version: "v2",
			},
			UpdateTime: time.Now(),
		}

		err = bgManagerBg2.Commit(originCtx, bgState)
		assertion.NoError(err)

		vhost, err = rabbitServiceBg2.FindVhostByClassifier(peerCtx, &classifierOrigin)
		assertion.Nil(vhost)

		//clone again with 6 enitites

		bgState = domain.BGState{
			Origin: &domain.BGNamespace{
				Name:    originNamespace,
				State:   "candidate",
				Version: "v3",
			},
			Peer: &domain.BGNamespace{
				Name:    peerNamespace,
				State:   "active",
				Version: "v2",
			},
			UpdateTime: time.Now(),
		}

		err = bgManagerBg2.Warmup(originCtx, bgState)
		assertion.NoError(err)

		vhost, err = rabbitServiceBg2.FindVhostByClassifier(originCtx, &classifierOrigin)
		assertion.NotNil(vhost)
		assertion.Equal("maas.origin.test-v3", vhost.Vhost)

		entities, err = rabbitServiceDaoBg2.GetRabbitEntitiesByVhostId(originCtx, vhost.Id)
		assertion.NoError(err)
		assertion.Equal(6, len(entities))
	})
}

func TestConfigV2RabbitBg2_RabbitWarmupVersEntErr(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		testInit(t, tdb, nil)

		authServiceMockBg2.EXPECT().CheckSecurityForBoundNamespaces(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
		authServiceMockBg2.EXPECT().CheckSecurityForSingleNamespace(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

		//initial config
		rabbitHelperMockBg2.EXPECT().
			CreateExchange(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "e1"}, "", nil).AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			CreateQueue(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{"name": "q1"}, "", nil)
		rabbitHelperMockBg2.EXPECT().
			CreateNormalOrLazyBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{"source": "e1"}, nil)

		rabbitHelperMockBg2.EXPECT().
			GetExchange(gomock.Any(), gomock.Any()).
			Return(nil, nil).
			AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			GetAllExchanges(gomock.Any()).
			Return(nil, nil).
			AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			CreateExchangeBinding(gomock.Any(), gomock.Any()).
			Return(&map[string]interface{}{}, "", nil).
			AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			DeleteExchange(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).
			AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			DeleteQueue(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).
			AnyTimes()
		rabbitHelperMockBg2.EXPECT().
			DeleteBinding(gomock.Any(), gomock.Any()).
			Return(map[string]interface{}{}, nil).
			AnyTimes()

		//binding will be created first, so it's also test for lazy binind

		cfg := `
---
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: origin
  services:
    - serviceName: order-processor
      config: |
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: origin
            versionedEntities:
                exchanges:
                - name: e2
                  type: direct                  
                queues:
                - name: q2
                bindings:
                - source: e2
                  destination: q2
            entities:
                exchanges:
                - name: e1
                  type: direct

`

		_, err := configServiceBg2.ApplyConfigV2(originCtx, cfg)
		assertion.NoError(err)

		bgState := domain.BGState{
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
		}

		err = bgManagerBg2.InitDomain(originCtx, bgState)
		assertion.NoError(err)

		err = bgManagerBg2.Warmup(originCtx, bgState)
		assertion.True(errors.Is(err, rabbit_service.ErrVersEntitiesExistsDuringWarmup))

		cfg = `
---
apiVersion: nc.maas.config/v2
kind: config
spec:
  version: v1
  namespace: origin
  services:
    - serviceName: order-processor
      config: |
        ---
        apiVersion: nc.maas.rabbit/v2
        kind: vhost
        spec:
            classifier:
              name: test
              namespace: origin 
            entities:
                exchanges:
                - name: e1
                  type: direct

`

		_, err = configServiceBg2.ApplyConfigV2(originCtx, cfg)
		assertion.NoError(err)

		err = bgManagerBg2.Warmup(originCtx, bgState)
		assertion.NoError(err)
	})
}

/*

several vhosts, no bg → bg

1. namespace1.vhost1 q1 non-exported, q2 non-exported
2. namespace1.vhost2 q1 non-exported, q2 non-exported → check no vhost exported, no change in vhost1
3. namespace1.vhost1 q1 exported, q2 non-exported → check namespace1.vhost1-exported q1-exported, no change in vhost2
4. namespace1.vhost1 q1 non-exported, q2 exported → check namespace1.vhost1-exported q1-non-exported, no change in vhost2

5. warmup →
- vhost1.namespace1.q1 non-exported, vhost1.namespace1.q2 exported,  vhost1.namespace2.q1 non-exported, vhost1.namespace2.q2 exported
- vhost2.namespace1.q1 non-exported, vhost2.namespace1.q2 non-exported,  vhost2.namespace2.q1 non-exported, vhost2.namespace2.q2 non-exported
6. vhost1.namespace1.q1 non-exported, vhost1.namespace1.q2 non-exported → check namespace1.vhost1-exported q2-exported (namespace2 still exported)
7. vhost1.namespace2.q1 non-exported, vhost1.namespace2.q2 non-exported → check namespace1.vhost1-exported q2 non-exported (deleted) (namespace2 not exported anymore)
— state:
- vhost1.namespace1.q1 non-exported, vhost1.namespace1.q2 non-exported,  vhost1.namespace2.q1 non-exported, vhost1.namespace2.q2 non-exported
- vhost2.namespace1.q1 non-exported, vhost2.namespace1.q2 non-exported,  vhost2.namespace2.q1 non-exported, vhost2.namespace2.q2 non-exported
8. (prepare for commit to check both namespace2 q2 exported (vhost1) and non-exported (vhost2) works correctly
vhost1.namespace2.q1 non-exported, vhost1.namespace2.q2 exported
vhost2.namespace1.q1 non-exported, vhost2.namespace1.q2 exported
— state:
- vhost1.namespace1.q1 non-exported, vhost1.namespace1.q2 non-exported,  vhost1.namespace2.q1 non-exported, vhost1.namespace2.q2 exported
- vhost2.namespace1.q1 non-exported, vhost2.namespace1.q2 exported,  vhost2.namespace2.q1 non-exported, vhost2.namespace2.q2 non-exported
9. commit →
- vhost1.namespace1.q1 non-exported, vhost1.namespace1.q2 non-exported (check no exported queue)
- vhost2.namespace1.q1 non-exported, vhost2.namespace1.q2 exported (check exported queue)

*/

func TestRegistrationService_ExportedVhostQueues(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		testharness.WithNewTestRabbit(t, func(rabbitHelperTest *testharness.TestRabbit) {

			testInit(t, tdb, rabbitHelperTest)

			classifierOrigin1 := model.Classifier{
				Name:      "vhost1",
				Namespace: originNamespace,
			}

			classifierOrigin2 := model.Classifier{
				Name:      "vhost2",
				Namespace: originNamespace,
			}

			classifierPeer1 := model.Classifier{
				Name:      "vhost1",
				Namespace: peerNamespace,
			}

			classifierExported1 := model.Classifier{
				Name:      "vhost1-exported",
				Namespace: originNamespace,
			}
			classifierExported2 := model.Classifier{
				Name:      "vhost2-exported",
				Namespace: originNamespace,
			}

			rabbitEntitites := model.RabbitEntities{
				Exchanges: []interface{}{
					map[string]interface{}{"name": "e1", "type": "direct"},
					map[string]interface{}{"name": "e2", "type": "direct"},
				},
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
					map[string]interface{}{"name": "q2"},
				},
				Bindings: []interface{}{
					map[string]interface{}{"source": "e1", "destination": "q1"},
					map[string]interface{}{"source": "e2", "destination": "q2"},
				},
			}

			configVhost1 := &model.RabbitConfigReqDto{
				Kind:   model.Kind{},
				Pragma: nil,
				Spec: model.ApplyRabbitConfigReqDto{
					Classifier: classifierOrigin1,
					Entities:   &rabbitEntitites,
				},
			}

			configVhost2 := &model.RabbitConfigReqDto{
				Kind:   model.Kind{},
				Pragma: nil,
				Spec: model.ApplyRabbitConfigReqDto{
					Classifier: classifierOrigin2,
					Entities:   &rabbitEntitites,
				},
			}

			_, err := applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)
			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost2, originNamespace)
			assertion.NoError(err)

			vhostExported1, err := rabbitWithHelper.FindVhostByClassifier(originCtx, &classifierExported1)
			assertion.NoError(err)
			assertion.Nil(vhostExported1)

			vhostExported2, err := rabbitWithHelper.FindVhostByClassifier(originCtx, &classifierExported2)
			assertion.NoError(err)
			assertion.Nil(vhostExported2)

			//phase 3
			rabbitEntitites = model.RabbitEntities{
				Queues: []interface{}{
					map[string]interface{}{"name": "q1", "exported": true},
					map[string]interface{}{"name": "q2"},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			entities, err := rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))

			shovels, err := rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "q1")

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Error(err) //no vhost2-exported

			//phase 4
			rabbitEntitites = model.RabbitEntities{
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
					map[string]interface{}{"name": "q2", "exported": true},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "q2")

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Error(err) //no vhost2-exported

			//phase 5 - warmup

			bgState := &domain.BGState{
				ControllerNamespace: "controller",
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
			}

			err = bgManagerBg2WithHelper.InitDomain(ctx, *bgState)
			assertion.NoError(err)

			err = bgManagerBg2WithHelper.Warmup(originCtx, *bgState)
			//err = rabbitWithHelper.Warmup(originCtx, bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(shovels))
			assertion.Contains(shovels[0].Name, "q2")
			assertion.Contains(shovels[1].Name, "q2")

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Error(err) //no vhost2-exported

			//phase 6 - get rid of one queue (second still there)
			rabbitEntitites = model.RabbitEntities{
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
					map[string]interface{}{"name": "q2"},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "q2")

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Error(err) //no vhost2-exported

			//phase 7 - get rid of second queue
			rabbitEntitites = model.RabbitEntities{
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
					map[string]interface{}{"name": "q2"},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites
			configVhost1.Spec.Classifier.Namespace = peerNamespace

			_, err = applyRabbitConfigurationWithRetry(peerCtx, configVhost1, peerNamespace)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(shovels))

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Error(err) //no vhost2-exported

			//phase 8 - prepare for commit to check both peer q2 exported (vhost1) and non-exported (vhost2) works correctly

			rabbitEntitites = model.RabbitEntities{
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
					map[string]interface{}{"name": "q2", "exported": true},
				},
			}

			//vhost1.peer.q1 non-exported, vhost1.peer.q2 exported
			configVhost1 = &model.RabbitConfigReqDto{
				Kind:   model.Kind{},
				Pragma: nil,
				Spec: model.ApplyRabbitConfigReqDto{
					Classifier: classifierPeer1,
					Entities:   &rabbitEntitites,
				},
			}

			//vhost2.origin.q1 non-exported, vhost2.origin.q2 exported
			configVhost2 = &model.RabbitConfigReqDto{
				Kind:   model.Kind{},
				Pragma: nil,
				Spec: model.ApplyRabbitConfigReqDto{
					Classifier: classifierOrigin2,
					Entities:   &rabbitEntitites,
				},
			}

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, peerNamespace)
			assertion.NoError(err)
			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost2, originNamespace)
			assertion.NoError(err)

			//phase 9 - commit
			bgState = &domain.BGState{
				ControllerNamespace: "controller",
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
			}

			err = rabbitWithHelper.Commit(originCtx, bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(shovels))

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported2)
			assertion.Equal(1, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(originCtx, classifierExported2)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "q2")

		})
	})
}

func TestRegistrationService_ExportedVhostExchanges(t *testing.T) {
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {
		testharness.WithNewTestRabbit(t, func(rabbitHelperTest *testharness.TestRabbit) {

			testInit(t, tdb, rabbitHelperTest)

			classifierOrigin1 := model.Classifier{
				Name:      "vhost1",
				Namespace: originNamespace,
			}

			classifierExported1 := model.Classifier{
				Name:      "vhost1-exported",
				Namespace: originNamespace,
			}

			rabbitEntitites := model.RabbitEntities{
				Exchanges: []interface{}{
					map[string]interface{}{"name": "e1", "type": "direct"},
				},
				Queues: []interface{}{
					map[string]interface{}{"name": "q1"},
				},
				Bindings: []interface{}{
					map[string]interface{}{"source": "e1", "destination": "q1"},
				},
			}

			configVhost1 := &model.RabbitConfigReqDto{
				Kind:   model.Kind{},
				Pragma: nil,
				Spec: model.ApplyRabbitConfigReqDto{
					Classifier: classifierOrigin1,
					Entities:   &rabbitEntitites,
				},
			}

			_, err := applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			vhostExported1, err := rabbitWithHelper.FindVhostByClassifier(originCtx, &classifierExported1)
			assertion.NoError(err)
			assertion.Nil(vhostExported1)

			rabbitEntitites = model.RabbitEntities{
				Exchanges: []interface{}{
					map[string]interface{}{"name": "e1", "exported": true},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			entities, err := rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))
			assertion.Equal("e1-origin-sq", entities.Queues[0].(map[string]interface{})["name"])
			assertion.Equal(3, len(entities.Bindings))
			bindings := convertBindingsToTypedArray(entities)

			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1-ae" && b.Destination == "e1-origin-sq"
				},
				),
			)
			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1" && b.Destination == "e1-origin-sq"
				},
				),
			)

			shovels, err := rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "e1")

			//warmup
			bgState := &domain.BGState{
				ControllerNamespace: "controller",
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
			}

			err = bgManagerBg2WithHelper.InitDomain(ctx, *bgState)
			assertion.NoError(err)

			err = bgManagerBg2WithHelper.Warmup(originCtx, *bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(entities.Queues))
			assertion.Equal("e1-origin-sq", entities.Queues[0].(map[string]interface{})["name"])
			assertion.Equal("e1-peer-sq", entities.Queues[1].(map[string]interface{})["name"])

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(shovels))
			assertion.Contains(shovels[0].Name, "e1-origin")
			assertion.Contains(shovels[1].Name, "e1-peer")

			bindings = convertBindingsToTypedArray(entities)

			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1-ae" && b.Destination == "e1-origin-sq"
				},
				),
			)
			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1" && b.Destination == "e1-origin-sq"
				},
				),
			)
			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1" && b.Destination == "e1-peer-sq"
				},
				),
			)

			// promote
			bgState = &domain.BGState{
				ControllerNamespace: "controller",
				Origin: &domain.BGNamespace{
					Name:    originNamespace,
					State:   "legacy",
					Version: "v1",
				},
				Peer: &domain.BGNamespace{
					Name:    peerNamespace,
					State:   "active",
					Version: "v2",
				},
				UpdateTime: time.Now(),
			}

			err = rabbitWithHelper.Promote(originCtx, bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(entities.Queues))
			bindings = convertBindingsToTypedArray(entities)
			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1-ae" && b.Destination == "e1-peer-sq"
				},
				),
			)

			shovels, err = rabbitWithHelper.GetShovels(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(shovels))

			// rollback
			bgState = &domain.BGState{
				ControllerNamespace: "controller",
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
			}

			err = rabbitWithHelper.Promote(originCtx, bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(entities.Queues))
			bindings = convertBindingsToTypedArray(entities)
			assertion.True(
				streams.From[Binding](bindings).AnyMatch(func(b Binding) bool {
					return b.Source == "e1-ae" && b.Destination == "e1-origin-sq"
				},
				),
			)

			shovels, err = rabbitWithHelper.GetShovels(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(2, len(shovels))

			//get rid of one queue (second still there)
			rabbitEntitites = model.RabbitEntities{
				Exchanges: []interface{}{
					map[string]interface{}{"name": "e1"},
				},
			}
			configVhost1.Spec.Entities = &rabbitEntitites

			_, err = applyRabbitConfigurationWithRetry(originCtx, configVhost1, originNamespace)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(entities.Queues))
			assertion.Equal("e1-peer-sq", entities.Queues[0].(map[string]interface{})["name"])

			shovels, err = rabbitWithHelper.GetShovels(ctx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(1, len(shovels))
			assertion.Contains(shovels[0].Name, "e1-peer")

			// commit
			bgState = &domain.BGState{
				ControllerNamespace: "controller",
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
			}

			err = rabbitWithHelper.Commit(originCtx, bgState)
			assertion.NoError(err)

			entities, err = rabbitWithHelper.GetConfig(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(entities.Queues))

			shovels, err = rabbitWithHelper.GetShovels(originCtx, classifierExported1)
			assertion.NoError(err)
			assertion.Equal(0, len(shovels))

		})
	})
}

func convertBindingsToTypedArray(entities *model.RabbitEntities) []Binding {
	var newBindings []Entity
	for _, binding := range entities.Bindings {
		newBindings = append(newBindings, binding.(map[string]interface{}))
	}

	bindings := streams.
		MapNonComparable[Entity, Binding](
		newBindings,
		func(x Entity) Binding {
			return Binding{
				Source:      x["source"].(string),
				Destination: x["destination"].(string),
			}
		})
	return bindings
}

func applyRabbitConfigurationWithRetry(ctx context.Context, cfg interface{}, namespace string) (interface{}, error) {
	log.DebugC(ctx, "Apply rabbit configuration with retry")
	return utils.RetryValue(ctx, 1*time.Minute, 5*time.Second, func(ctx context.Context) (interface{}, error) {
		return configWithRabbit.ApplyRabbitConfiguration(ctx, cfg, namespace)
	})
}
