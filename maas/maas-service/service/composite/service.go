package composite

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/utils"
	"golang.org/x/exp/slices"
)

//go:generate mockgen -source=service.go -destination=service_mock.go -package composite
type RegistrationDao interface {
	Upsert(ctx context.Context, registrationEntity *CompositeRegistration) error
	GetByBaseline(ctx context.Context, baseline string) (*CompositeRegistration, error)
	GetByNamespace(ctx context.Context, namespace string) (*CompositeRegistration, error)
	List(ctx context.Context) ([]CompositeRegistration, error)
	DeleteByBaseline(ctx context.Context, baseline string) error
}

type RegistrationService struct {
	registrationDao RegistrationDao
}

func NewRegistrationService(registrationDao RegistrationDao) *RegistrationService {
	return &RegistrationService{registrationDao: registrationDao}
}

func (s *RegistrationService) Upsert(ctx context.Context, registrationRequest *CompositeRegistration) error {
	return s.registrationDao.Upsert(ctx, registrationRequest)
}

func (s *RegistrationService) GetByBaseline(ctx context.Context, baseline string) (*CompositeRegistration, error) {
	return s.registrationDao.GetByBaseline(ctx, baseline)
}

func (s *RegistrationService) GetByNamespace(ctx context.Context, namespace string) (*CompositeRegistration, error) {
	return s.registrationDao.GetByNamespace(ctx, namespace)
}

func (s *RegistrationService) List(ctx context.Context) ([]CompositeRegistration, error) {
	return s.registrationDao.List(ctx)
}

func (s *RegistrationService) Destroy(ctx context.Context, baseline string) error {
	return s.registrationDao.DeleteByBaseline(ctx, baseline)
}

func (s *RegistrationService) CleanupNamespace(ctx context.Context, namespace string) error {
	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		log.InfoC(ctx, "Remove namespace `%v' from composite registration...", namespace)

		registration, err := s.registrationDao.GetByNamespace(ctx, namespace)
		if err != nil {
			return err
		}
		if registration == nil {
			// not a member of any composite
			log.InfoC(ctx, "Namespace `%s' is not a member of any composite. Skip removal", namespace)
			return nil
		}

		// if removing namespace is baseline origin (=compositeId), then destroy whole composite
		if registration.Id == namespace {
			log.InfoC(ctx, "Destroy composite: %+v", registration)
			if err = s.registrationDao.DeleteByBaseline(ctx, registration.Id); err != nil {
				return utils.LogError(log, ctx, "error destroy composite %+v: %w", registration, err)
			}
		} else {
			log.InfoC(ctx, "Remove member namespace `%s' from composite: %+v", namespace, registration)
			registration.Namespaces = slices.DeleteFunc(registration.Namespaces, func(ns string) bool { return ns == namespace })
			if err = s.registrationDao.Upsert(ctx, registration); err != nil {
				return utils.LogError(log, ctx, "error update composite %+v: %w", registration, err)
			}
		}

		return nil
	})
}
