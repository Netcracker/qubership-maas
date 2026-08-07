package composite

import (
	"context"
	"fmt"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/stretchr/testify/assert"
)

func TestRegistrationService_Upsert(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	registrationDao.EXPECT().
		Upsert(gomock.Any(), gomock.Eq(&CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}})).Return(nil)

	err := registrationService.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}})
	assert.NoError(t, err)
}

func TestRegistrationService_Upsert_ModifyIndex(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	registration := &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(42))}
	registrationDao.EXPECT().Upsert(gomock.Any(), gomock.Eq(registration)).Return(nil)

	assert.NoError(t, registrationService.Upsert(ctx, registration))

	// modify index rejection must be propagated to the caller as is
	stale := &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(1))}
	registrationDao.EXPECT().Upsert(gomock.Any(), gomock.Eq(stale)).
		Return(fmt.Errorf("new modify index '1' cannot be less than the current index '42': %w", msg.BadRequest))

	err := registrationService.Upsert(ctx, stale)
	assert.ErrorIs(t, err, msg.BadRequest)
}

func TestRegistrationService_GetByBaseline(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	expected := &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}
	registrationDao.EXPECT().GetByBaseline(gomock.Any(), "a").Return(expected, nil)

	actual, err := registrationService.GetByBaseline(ctx, "a")
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRegistrationService_List(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	expected := []CompositeRegistration{{Id: "a", Namespaces: []string{"a", "b"}}}
	registrationDao.EXPECT().List(gomock.Any()).Return(expected, nil)

	actual, err := registrationService.List(ctx)
	assert.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestRegistrationService_Destroy(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	registrationDao.EXPECT().DeleteByBaseline(gomock.Any(), gomock.Eq("a")).Return(nil)

	err := registrationService.Destroy(ctx, "a")
	assert.NoError(t, err)
}

func TestRegistrationService_CleanupNamespace_RemoveMemberFromComposite(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	gomock.InOrder(
		registrationDao.EXPECT().
			GetByNamespace(gomock.Any(), "b").
			Return(&CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}, nil),
		registrationDao.EXPECT().
			Upsert(gomock.Any(), gomock.Eq(&CompositeRegistration{Id: "a", Namespaces: []string{"a"}})).Return(nil),
	)

	err := registrationService.CleanupNamespace(ctx, "b")
	assert.NoError(t, err)
}

// remove composite id member
func TestRegistrationService_CleanupNamespace_DestroyComposite(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	gomock.InOrder(
		registrationDao.EXPECT().
			GetByNamespace(gomock.Any(), "a").
			Return(&CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}, nil),
		registrationDao.EXPECT().
			DeleteByBaseline(gomock.Any(), gomock.Eq("a")).Return(nil),
	)

	err := registrationService.CleanupNamespace(ctx, "a")
	assert.NoError(t, err)
}

func TestRegistrationService_CleanupNamespace_NotInComposite(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	registrationDao := NewMockRegistrationDao(mockCtrl)
	registrationService := NewRegistrationService(registrationDao)

	registrationDao.EXPECT().
		GetByNamespace(gomock.Any(), "b").
		Return(nil, nil)

	err := registrationService.CleanupNamespace(ctx, "b")
	assert.NoError(t, err)
}
