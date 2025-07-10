package domain

import (
	"context"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
)

//go:generate mockgen -source=bg_domain_service.go -destination=mock/bg_domain_service_mock.go
type BGDomainService interface {
	Bind(ctx context.Context, ns1 string, ns2 string, ctr string) error
	List(ctx context.Context) ([]BGNamespaces, error)
	FindByNamespace(ctx context.Context, namespace string) (*BGNamespaces, error)
	Unbind(ctx context.Context, namespace string) (*BGNamespaces, error)
	UpdateController(ctx context.Context, origin, controller string) error

	InsertBgState(ctx context.Context, state BGState) error
	GetCurrentBgStateByNamespace(ctx context.Context, namespace string) (*BGState, error)
}

type BGDomainDao interface {
	List(ctx context.Context) (*[]BGDomainEntity, error)
	Bind(ctx context.Context, origin string, peer string, controller string) error
	FindByNamespace(ctx context.Context, namespace string) (*BGDomainEntity, error)
	UpdateController(ctx context.Context, origin string, controller string) error
	Unbind(ctx context.Context, namespace string) (*BGDomainEntity, error)

	UpdateState(ctx context.Context, bgStateEntity BGStateEntity) error
	GetState(ctx context.Context, namespace int64) (*BGStateEntity, error)
}

type BGDomainServiceImpl struct {
	dao BGDomainDao
}

func NewBGDomainService(dao BGDomainDao) *BGDomainServiceImpl {
	return &BGDomainServiceImpl{dao: dao}
}

func (bgds *BGDomainServiceImpl) List(ctx context.Context) ([]BGNamespaces, error) {
	entities, err := bgds.dao.List(ctx)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error listing bg domains: %w", err)
	}

	registrations := make([]BGNamespaces, 0)
	for _, entity := range *entities {
		registration := BGNamespaces{
			Origin:              entity.Origin,
			Peer:                entity.Peer,
			ControllerNamespace: entity.Controller.String,
		}

		registrations = append(registrations, registration)
	}

	return registrations, nil
}

func (bgds *BGDomainServiceImpl) Bind(ctx context.Context, origin string, peer string, controller string) error {
	log.InfoC(ctx, "Create BG Domain by: %v and %v", origin, peer)
	if err := bgds.dao.Bind(ctx, origin, peer, controller); err == nil {
		log.InfoC(ctx, "BG Domain successfully created")
		return nil
	} else {
		return utils.LogError(log, ctx, "error create domain for: (%v, %v), error: %w", origin, peer, err)
	}
}

// its migration hack to add controller namespace missed on first bg2 implementation
func (bgds *BGDomainServiceImpl) UpdateController(ctx context.Context, origin string, controller string) error {
	log.InfoC(ctx, "Update BG Domain: %v, by adding controller ns: %v", origin, controller)
	if err := bgds.dao.UpdateController(ctx, origin, controller); err == nil {
		log.InfoC(ctx, "Controller namespace added")
		return nil
	} else {
		return utils.LogError(log, ctx, "error update domain controller: %v, error: %w", origin, err)
	}
}

func (bgds *BGDomainServiceImpl) FindByNamespace(ctx context.Context, namespace string) (*BGNamespaces, error) {
	log.DebugC(ctx, "Search for bg domain by: %v", namespace)
	if res, err := bgds.dao.FindByNamespace(ctx, namespace); err == nil {
		if res != nil {
			log.DebugC(ctx, "Domain found: %+v", res)
			return res.toValue(), nil
		} else {
			log.DebugC(ctx, "Domain not found by: %v", namespace)
			return nil, nil
		}
	} else {
		return nil, utils.LogError(log, ctx, "Error search bg domain registration: %w", err)
	}
}

// Unbind remove any namespace included in domain, will destroy whole domain
func (bgds *BGDomainServiceImpl) Unbind(ctx context.Context, namespace string) (*BGNamespaces, error) {
	log.InfoC(ctx, "Destroy domain by namespace: %v", namespace)
	if domain, err := bgds.dao.Unbind(ctx, namespace); err == nil {
		log.InfoC(ctx, "BG Domain successfully destroyed: %+v", domain)
		return domain.toValue(), nil
	} else {
		return nil, utils.LogError(log, ctx, "error destroy BG Domain by: %v, error: %w", namespace, err)
	}
}

func (bgds *BGDomainServiceImpl) InsertBgState(ctx context.Context, state BGState) error {
	log.InfoC(ctx, "Update bg domain state: %+v", state)
	domain, err := bgds.dao.FindByNamespace(ctx, state.Origin.Name)
	if err != nil {
		return utils.LogError(log, ctx, "error locate bg domain: %w", err)
	}
	if domain == nil {
		return utils.LogError(log, ctx, "can't locate registered bgdomain for namespace: %v: %w", state.Origin.Name, msg.BadRequest)
	}
	// additional check to test structure validity
	if domain.Origin != state.Origin.Name || domain.Peer != state.Peer.Name {
		return utils.LogError(log, ctx, "unmatched bgdomain namespaces for state and registered domain: domain=%+v, state=%+v: %w", domain, state, msg.BadRequest)
	}

	if err := bgds.dao.UpdateState(ctx, BGStateEntity{
		BgDomainId: domain.ID,
		OriginNs:   *state.Origin,
		PeerNs:     *state.Peer,
		UpdateTime: state.UpdateTime,
	}); err == nil {
		log.InfoC(ctx, "BGDomain state was successfully updated")
		return nil
	} else {
		return utils.LogError(log, ctx, "error update bgdomain state: %w", err)
	}
}

func (bgds *BGDomainServiceImpl) GetCurrentBgStateByNamespace(ctx context.Context, namespace string) (*BGState, error) {
	domain, err := bgds.dao.FindByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error locate bg domain: %w", err)
	}
	if domain == nil {
		return nil, nil
	}

	stateEntity, err := bgds.dao.GetState(ctx, domain.ID)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error during GetCurrentBgStateByNamespace: %w", err)
	}
	if stateEntity == nil {
		return nil, nil
	}

	return &BGState{
		ControllerNamespace: domain.Controller.String,
		Origin:              &stateEntity.OriginNs,
		Peer:                &stateEntity.PeerNs,
		UpdateTime:          stateEntity.UpdateTime,
	}, nil
}
