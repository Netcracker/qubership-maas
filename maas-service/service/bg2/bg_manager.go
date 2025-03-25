package bg2

import (
	"context"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"maas/maas-service/dao"
	"maas/maas-service/msg"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/utils"
)

var log = logging.GetLogger("bg-manager")

//go:generate mockgen -source=bg_manager.go -destination=mock/bg_manager.go
type BgManager struct {
	bgDomainService domain.BGDomainService
	brokerServices  []BrokerService
}

type BrokerService interface {
	Warmup(context.Context, *domain.BGState) error
	Commit(context.Context, *domain.BGState) error
	Promote(context.Context, *domain.BGState) error
	Rollback(context.Context, *domain.BGState) error
	DestroyDomain(context.Context, *domain.BGNamespaces) error
}

func NewManager(bgDomainService domain.BGDomainService, brokerServices ...BrokerService) *BgManager {
	return &BgManager{bgDomainService: bgDomainService, brokerServices: brokerServices}
}

func (m *BgManager) InitDomain(ctx context.Context, state domain.BGState) error {
	log.InfoC(ctx, "Starting to init domain: %+v", state)
	pair, err := m.bgDomainService.FindByNamespace(ctx, state.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "error find existing domain by: %+v, error: %w", state, err)
	}
	if pair != nil && pair.Origin == state.Origin.Name && pair.Peer == state.Peer.Name {
		log.InfoC(ctx, "Domain already registered: %+v, Skipping", state)
		return nil
	}

	if err := m.bgDomainService.Bind(ctx, state.Origin.Name, state.Peer.Name, state.ControllerNamespace); err != nil {
		return utils.LogError(log, ctx, "error registering domain: %w", err)
	}

	log.InfoC(ctx, "Domain %+v successfully initialized", state)
	return nil
}

func (m *BgManager) Warmup(ctx context.Context, bgState domain.BGState) error {
	log.InfoC(ctx, "Starting to warmup: %+v", bgState)

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return m.validateDomain(ctx, bgState.Origin.Name, bgState.Peer.Name, func() error {
			//update domain for migration case when controller was not set
			if err := m.bgDomainService.UpdateController(ctx, bgState.Origin.Name, bgState.ControllerNamespace); err != nil {
				return utils.LogError(log, ctx, "error update controller domain: %w", err)
			}

			if err := m.bgDomainService.InsertBgState(ctx, bgState); err != nil {
				return utils.LogError(log, ctx, "error during InsertBgState: %w", err)
			}

			for _, service := range m.brokerServices {
				log.InfoC(ctx, "warmup service: %+v", service)
				if err := service.Warmup(ctx, &bgState); err != nil {
					return utils.LogError(log, ctx, "error warmup service, error: %w", err)
				}
			}

			return nil
		})
	})
}

func (m *BgManager) Commit(ctx context.Context, bgState domain.BGState) error {
	log.InfoC(ctx, "Starting to commit: %+v", bgState)

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return m.validateDomain(ctx, bgState.Origin.Name, bgState.Peer.Name, func() error {
			if err := m.bgDomainService.InsertBgState(ctx, bgState); err != nil {
				return utils.LogError(log, ctx, "error during InsertBgState: %w", err)
			}

			for _, service := range m.brokerServices {
				log.InfoC(ctx, "Commit to service: %+v, %+v", service, bgState)
				if err := service.Commit(ctx, &bgState); err != nil {
					return utils.LogError(log, ctx, "error Commit to service %v: %w", service, err)
				}
			}

			log.InfoC(ctx, "Commit for %+v successfully finished", bgState)
			return nil
		})
	})
}

func (m *BgManager) Promote(ctx context.Context, bgState domain.BGState) error {
	log.InfoC(ctx, "Starting to promote: %+v", bgState)

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return m.validateDomain(ctx, bgState.Origin.Name, bgState.Peer.Name, func() error {
			if err := m.bgDomainService.InsertBgState(ctx, bgState); err != nil {
				return utils.LogError(log, ctx, "error during InsertBgState: %w", err)
			}

			for _, service := range m.brokerServices {
				log.InfoC(ctx, "Promote to service: %+v, %+v", service, bgState)
				if err := service.Promote(ctx, &bgState); err != nil {
					return utils.LogError(log, ctx, "error Promote to service %v: %w", service, err)
				}
			}

			log.InfoC(ctx, "Promote for %+v successfully finished", bgState)
			return nil
		})
	})
}

func (m *BgManager) Rollback(ctx context.Context, bgState domain.BGState) error {
	log.InfoC(ctx, "Starting to rollback: %+v", bgState)

	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return m.validateDomain(ctx, bgState.Origin.Name, bgState.Peer.Name, func() error {
			if err := m.bgDomainService.InsertBgState(ctx, bgState); err != nil {
				return utils.LogError(log, ctx, "error during InsertBgState: %w", err)
			}

			for _, service := range m.brokerServices {
				log.InfoC(ctx, "Rollback to service: %+v, %+v", service, bgState)
				if err := service.Rollback(ctx, &bgState); err != nil {
					return utils.LogError(log, ctx, "error Rollback to service %v: %w", service, err)
				}
			}

			log.InfoC(ctx, "Rollback for %+v successfully finished", bgState)
			return nil
		})
	})
}

func (m *BgManager) DestroyDomain(ctx context.Context, namespaces domain.BGNamespaces) error {
	log.InfoC(ctx, "Destroy domain: %+v", namespaces)
	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		return m.validateDomain(ctx, namespaces.Origin, namespaces.Peer, func() error {
			for _, service := range m.brokerServices {
				log.InfoC(ctx, "Destroy domain to service: %+v, %+v", service, namespaces)
				if err := service.DestroyDomain(ctx, &namespaces); err != nil {
					return utils.LogError(log, ctx, "error destroy domain %+v: %w", namespaces, err)
				}
			}

			if _, err := m.bgDomainService.Unbind(ctx, namespaces.Origin); err != nil {
				return utils.LogError(log, ctx, "error unregister domain: %+v, %w", namespaces, err)
			}

			log.InfoC(ctx, "Destroy domain %+v successfully finished", namespaces)
			return nil
		})
	})
}

func (m *BgManager) ListDomains(ctx context.Context) ([]domain.BGNamespaces, error) {
	log.InfoC(ctx, "List bg domains")
	return m.bgDomainService.List(ctx)
}

func (m *BgManager) validateDomain(ctx context.Context, origin, peer string, f func() error) error {
	// check that such domain exists
	if domain, err := m.bgDomainService.FindByNamespace(ctx, origin); err == nil {
		if domain == nil || (domain.Origin != origin || domain.Peer != peer) {
			return utils.LogError(log, ctx, "domain with such namespace pair (%v,%v) is not registered: %w", origin, peer, msg.NotFound)
		}
	} else {
		return utils.LogError(log, ctx, "error getting domain registration: %w", err)
	}

	return f()
}
