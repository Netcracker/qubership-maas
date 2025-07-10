package bg2

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	mock_domain "github.com/netcracker/qubership-maas/service/bg2/domain/mock"
	mock_bg2 "github.com/netcracker/qubership-maas/service/bg2/mock"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestBgManager_API(t *testing.T) {
	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		bgDomainService := domain.NewBGDomainService(domain.NewBGDomainDao(base))

		mockCtrl := gomock.NewController(t)

		brokerService := mock_bg2.NewMockBrokerService(mockCtrl)
		brokerService.EXPECT().Warmup(gomock.Any(), gomock.Any()).AnyTimes()
		brokerService.EXPECT().Commit(gomock.Any(), gomock.Any()).AnyTimes()

		manager := NewManager(bgDomainService, brokerService)

		// ==========================================================
		// incorrect sequence, call Wamup before init domain
		// ==========================================================
		{
			bgState := domain.BGState{Origin: &domain.BGNamespace{Name: "primary"}, Peer: &domain.BGNamespace{Name: "secondary"}}
			assert.Error(t, manager.Warmup(ctx, bgState))
		}

		// ==========================================================
		// INIT_DOMAIN
		// ==========================================================
		{
			bgState := domain.BGState{Origin: &domain.BGNamespace{Name: "primary"}, Peer: &domain.BGNamespace{Name: "secondary"}}
			assert.NoError(t, manager.InitDomain(ctx, bgState))
			// test reenter-ability
			assert.NoError(t, manager.InitDomain(ctx, bgState))

		}

		layout := "2006-01-02 15:04:05"
		// ==========================================================
		// WARMUP
		// ==========================================================
		{
			ts, _ := time.Parse(layout, "2006-01-02 15:04:05")
			bgState := domain.BGState{
				Origin: &domain.BGNamespace{
					Name:    "primary",
					State:   "active",
					Version: "v1",
				},
				Peer: &domain.BGNamespace{
					Name:    "secondary",
					State:   "candidate",
					Version: "v2",
				},
				UpdateTime: ts,
			}
			assert.NoError(t, manager.Warmup(ctx, bgState))

		}

		// ==========================================================
		// COMMIT
		// ==========================================================
		{
			ts, _ := time.Parse(layout, "2006-02-02 15:04:05")
			bgState := domain.BGState{
				Origin: &domain.BGNamespace{
					Name:    "primary",
					State:   "active",
					Version: "v1",
				},
				Peer: &domain.BGNamespace{
					Name:    "secondary",
					State:   "idle",
					Version: "null",
				},
				UpdateTime: ts,
			}
			assert.NoError(t, manager.Commit(ctx, bgState))
		}

	})
}

func TestBgManager_DestroyDomain_Ok(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	bgdms := mock_domain.NewMockBGDomainService(mockCtrl)
	brokerService := mock_bg2.NewMockBrokerService(mockCtrl)
	bgm := NewManager(bgdms, brokerService)

	bgdms.EXPECT().FindByNamespace(gomock.Any(), gomock.Eq("ns1")).
		Return(&domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"}, nil).
		Times(1)

	brokerService.EXPECT().DestroyDomain(gomock.Any(), gomock.Any()).Times(1)
	bgdms.EXPECT().Unbind(gomock.Any(), gomock.Eq("ns1")).Times(1)

	err := bgm.DestroyDomain(ctx, domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"})

	assert.NoError(t, err)
}

func TestBgManager_DestroyDomain_BrokerServiceError(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	bgdms := mock_domain.NewMockBGDomainService(mockCtrl)
	brokerService := mock_bg2.NewMockBrokerService(mockCtrl)
	bgm := NewManager(bgdms, brokerService)

	bgdms.EXPECT().FindByNamespace(gomock.Any(), gomock.Eq("ns1")).
		Return(&domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"}, nil).
		Times(1)

	brokerService.EXPECT().DestroyDomain(gomock.Any(), gomock.Any()).
		Return(msg.Conflict).
		Times(1)

	err := bgm.DestroyDomain(ctx, domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"})

	assert.ErrorIs(t, err, msg.Conflict)
}

func TestBgManager_DestroyDomain_InvalidDomain(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	bgdms := mock_domain.NewMockBGDomainService(mockCtrl)
	bgm := NewManager(bgdms, nil, nil)

	bgdms.EXPECT().FindByNamespace(gomock.Any(), gomock.Eq("ns1")).
		Return(&domain.BGNamespaces{Origin: "ns1", Peer: "other", ControllerNamespace: "ns3"}, nil).
		Times(1)

	err := bgm.DestroyDomain(ctx, domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"})

	assert.ErrorIs(t, err, msg.NotFound)
}

func TestBgManager_DestroyDomain_NoSuchDomain(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)
	bgdms := mock_domain.NewMockBGDomainService(mockCtrl)
	bgm := NewManager(bgdms, nil, nil)

	bgdms.EXPECT().FindByNamespace(gomock.Any(), gomock.Eq("ns1")).Return(nil, nil).Times(1)
	err := bgm.DestroyDomain(ctx, domain.BGNamespaces{Origin: "ns1", Peer: "ns2", ControllerNamespace: "ns3"})

	assert.ErrorIs(t, err, msg.NotFound)
}
