package composite

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"testing"
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
