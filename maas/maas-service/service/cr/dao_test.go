package cr

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"testing"
)

func TestDefaultWaitListDao_Create(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)
		trackingId := rand.Int63()

		waitEntity, err := waitListDao.GetByTrackingId(ctx, trackingId)
		assert.NoError(t, err)
		assert.Nil(t, waitEntity)

		resourceWaitEntity := CustomResourceWaitEntity{
			TrackingId:     trackingId,
			CustomResource: "{\"test-custom-resource\": \"resource\"}",
			Namespace:      "test-namespace",
			Reason:         "test-reason",
			Status:         CustomResourceStatusInProgress,
		}
		err = waitListDao.Create(ctx, &resourceWaitEntity)
		assert.NoError(t, err)

		waitEntity, err = waitListDao.GetByTrackingId(ctx, trackingId)
		assert.NoError(t, err)
		assert.NotNil(t, waitEntity)
		assert.Equal(t, trackingId, waitEntity.TrackingId)
		assert.Equal(t, "{\"test-custom-resource\": \"resource\"}", waitEntity.CustomResource)
		assert.Equal(t, "test-namespace", waitEntity.Namespace)
		assert.Equal(t, "test-reason", waitEntity.Reason)
		assert.Equal(t, CustomResourceStatusInProgress, waitEntity.Status)
		assert.NotNil(t, waitEntity.CreatedAt)
	})
}

func TestDefaultWaitListDao_Create_Fills_Track_Id(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)

		resourceWaitEntity := CustomResourceWaitEntity{
			CustomResource: "{\"Metadata\": {\"Name\": \"test-name\", \"Namespace\": \"test-namespace\"}}",
			Namespace:      "test-namespace",
			Reason:         "test-reason",
			Status:         CustomResourceStatusInProgress,
		}
		err := waitListDao.Create(ctx, &resourceWaitEntity)
		assert.NoError(t, err)
		assert.NotZero(t, resourceWaitEntity.TrackingId)

		trackingId := resourceWaitEntity.TrackingId

		resourceWaitEntity = CustomResourceWaitEntity{
			CustomResource: "{\"Metadata\": {\"Name\": \"test-name\", \"Namespace\": \"test-namespace\"}}",
			Namespace:      "test-namespace",
			Reason:         "test-reason",
			Status:         CustomResourceStatusInProgress,
		}
		err = waitListDao.Create(ctx, &resourceWaitEntity)
		assert.NoError(t, err)
		assert.Equal(t, trackingId, resourceWaitEntity.TrackingId)
	})
}

func TestDefaultWaitListDao_DeleteByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)

		err := waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "first"))
		assert.NoError(t, err)

		err = waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "first"))
		assert.NoError(t, err)

		err = waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "second"))
		assert.NoError(t, err)

		waitEntities, err := waitListDao.FindByNamespaceAndStatus(ctx, "first", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 2)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "second", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 1)

		err = waitListDao.DeleteByNamespace(ctx, "first")
		assert.NoError(t, err)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "second", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 1)
		assert.Equal(t, "second", waitEntities[0].Namespace)
	})
}

func TestPGWaitListDao_DeleteByNameAndNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)

		trackingIdToDelete := rand.Int63()
		err := waitListDao.Create(ctx, newResourceWaitEntityWithName(trackingIdToDelete, "test-name", "first"))
		assert.NoError(t, err)

		err = waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "first"))
		assert.NoError(t, err)

		err = waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "second"))
		assert.NoError(t, err)

		waitEntities, err := waitListDao.FindByNamespaceAndStatus(ctx, "first", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 2)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "second", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 1)

		err = waitListDao.DeleteByNameAndNamespace(ctx, "test-name", "first")
		assert.NoError(t, err)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "first", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 1)
		assert.Equal(t, "first", waitEntities[0].Namespace)
		assert.NotEqual(t, trackingIdToDelete, waitEntities[0].TrackingId)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "second", CustomResourceStatusInProgress)
		assert.NoError(t, err)
		assert.Len(t, waitEntities, 1)
		assert.Equal(t, "second", waitEntities[0].Namespace)
	})
}

func TestDefaultWaitListDao_FindByNamespaceAndStatus(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)

		firstInFirst := newResourceWaitEntity(rand.Int63(), "first")
		firstInFirst.Status = CustomResourceStatusCompleted
		err := waitListDao.Create(ctx, firstInFirst)
		assert.NoError(t, err)

		secondInFirst := newResourceWaitEntity(rand.Int63(), "first")
		secondInFirst.Status = CustomResourceStatusTerminated
		err = waitListDao.Create(ctx, secondInFirst)
		assert.NoError(t, err)

		err = waitListDao.Create(ctx, newResourceWaitEntity(rand.Int63(), "second"))
		assert.NoError(t, err)

		waitEntities, err := waitListDao.FindByNamespaceAndStatus(ctx, "first", CustomResourceStatusTerminated)
		assert.NoError(t, err)
		assert.NotNil(t, waitEntities)
		assert.Len(t, waitEntities, 1)
		assert.Equal(t, "first", waitEntities[0].Namespace)
		assert.Equal(t, CustomResourceStatusTerminated, waitEntities[0].Status)

		waitEntities, err = waitListDao.FindByNamespaceAndStatus(ctx, "first", CustomResourceStatusTerminated, CustomResourceStatusCompleted)
		assert.NoError(t, err)
		assert.NotNil(t, waitEntities)
		assert.Len(t, waitEntities, 2)
		assert.Equal(t, "first", waitEntities[0].Namespace)
		assert.Equal(t, CustomResourceStatusCompleted, waitEntities[0].Status)
		assert.Equal(t, "first", waitEntities[1].Namespace)
		assert.Equal(t, CustomResourceStatusTerminated, waitEntities[1].Status)
	})
}

func TestDefaultWaitListDao_GetByTrackId(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)
		trackingId := rand.Int63()

		resourceWaitEntityFromDb, err := waitListDao.GetByTrackingId(ctx, trackingId)
		assert.NoError(t, err)
		assert.Nil(t, resourceWaitEntityFromDb)

		resourceWaitEntity := newResourceWaitEntity(trackingId, "test-namespace")
		err = waitListDao.Create(ctx, resourceWaitEntity)
		assert.NoError(t, err)

		resourceWaitEntityFromDb, err = waitListDao.GetByTrackingId(ctx, trackingId)
		assert.NoError(t, err)
		assert.NotNil(t, resourceWaitEntityFromDb)
		assert.Equal(t, resourceWaitEntity.TrackingId, resourceWaitEntityFromDb.TrackingId)
	})
}

func TestDefaultWaitListDao_UpdateStatus(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		waitListDao := NewPGWaitListDao(baseDao)
		trackingId := rand.Int63()

		resourceWaitEntity := newResourceWaitEntity(trackingId, "test-namespace")
		err := waitListDao.Create(ctx, resourceWaitEntity)
		assert.NoError(t, err)

		resourceWaitEntityFromDb, err := waitListDao.GetByTrackingId(ctx, trackingId)
		assert.NoError(t, err)
		assert.NotNil(t, resourceWaitEntityFromDb)
		assert.Equal(t, resourceWaitEntity.TrackingId, resourceWaitEntityFromDb.TrackingId)

		resourceWaitEntityFromDb, err = waitListDao.UpdateStatus(ctx, trackingId, CustomResourceStatusCompleted)
		assert.NoError(t, err)
		assert.NotNil(t, resourceWaitEntityFromDb)
		assert.Equal(t, resourceWaitEntity.TrackingId, resourceWaitEntityFromDb.TrackingId)
		assert.Equal(t, "test-reason", resourceWaitEntityFromDb.Reason)
		assert.Equal(t, resourceWaitEntity.CustomResource, resourceWaitEntityFromDb.CustomResource)
		assert.Equal(t, CustomResourceStatusCompleted, resourceWaitEntityFromDb.Status)

		resourceWaitEntityFromDb, err = waitListDao.UpdateStatusAndReason(ctx, trackingId, CustomResourceStatusFailed, "new-reason")
		assert.NoError(t, err)
		assert.NotNil(t, resourceWaitEntityFromDb)
		assert.Equal(t, resourceWaitEntity.TrackingId, resourceWaitEntityFromDb.TrackingId)
		assert.Equal(t, "new-reason", resourceWaitEntityFromDb.Reason)
		assert.Equal(t, resourceWaitEntity.CustomResource, resourceWaitEntityFromDb.CustomResource)
		assert.Equal(t, CustomResourceStatusFailed, resourceWaitEntityFromDb.Status)
	})
}

func newResourceWaitEntity(trackingId int64, namespace string) *CustomResourceWaitEntity {
	return newResourceWaitEntityWithName(trackingId, uuid.NewString(), namespace)
}

func newResourceWaitEntityWithName(trackingId int64, name, namespace string) *CustomResourceWaitEntity {
	return &CustomResourceWaitEntity{
		TrackingId:     trackingId,
		CustomResource: fmt.Sprintf("{\"Metadata\": {\"Name\": \"%s\", \"Namespace\": \"%s\"}}", name, namespace),
		Namespace:      namespace,
		Reason:         "test-reason",
		Status:         CustomResourceStatusInProgress,
	}
}
