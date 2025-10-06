package monitoring

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDaoImpl_GetAllEntityRequestsStat(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx := context.Background()
		serviceDao := NewDao(baseDao)

		stats, err := serviceDao.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Empty(t, stats)

		stat := &EntityRequestStat{
			Namespace:    "test-namespace",
			Microservice: "test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		}
		err = serviceDao.AddEntityRequestStat(ctx, stat)
		assert.NoError(t, err)

		stats, err = serviceDao.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(*stats))

		first := (*stats)[0]
		assert.Equal(t, "test-namespace", first.Namespace)
		assert.Equal(t, "test-microservice", first.Microservice)
		assert.Equal(t, "test-topic", first.Name)
		assert.Equal(t, EntityTypeTopic, first.EntityType)
		assert.Equal(t, int64(1), first.RequestsTotal)
		// FIXME something wrong with time zone
		//assert.True(t, time.Now().Sub(first.LastRequestTs).Minutes() < 1)
	})
}

func TestDaoImpl_AddEntityRequestStat(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao2 := NewDao(baseDao)

		stats, err := dao2.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Empty(t, stats)

		stat := &EntityRequestStat{
			Namespace:    "test-namespace",
			Microservice: "test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		}
		err = dao2.AddEntityRequestStat(ctx, stat)
		assert.NoError(t, err)

		stats, err = dao2.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(*stats))
		assert.Equal(t, int64(1), (*stats)[0].RequestsTotal)

		err = dao2.AddEntityRequestStat(ctx, stat)
		assert.NoError(t, err)

		stats, err = dao2.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(*stats))
		assert.Equal(t, int64(2), (*stats)[0].RequestsTotal)

		anotherStat := &EntityRequestStat{
			Namespace:    "another-test-namespace",
			Microservice: "test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		}
		err = dao2.AddEntityRequestStat(ctx, anotherStat)
		assert.NoError(t, err)

		stats, err = dao2.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 2, len(*stats))
	})
}

func TestDaoImpl_DeleteEntityRequestsStatByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		serviceDao := NewDao(baseDao)

		err := serviceDao.AddEntityRequestStat(ctx, &EntityRequestStat{
			Namespace:    "test-namespace",
			Microservice: "test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		})
		assert.NoError(t, err)

		err = serviceDao.AddEntityRequestStat(ctx, &EntityRequestStat{
			Namespace:    "test-namespace",
			Microservice: "another-test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		})
		assert.NoError(t, err)

		err = serviceDao.AddEntityRequestStat(ctx, &EntityRequestStat{
			Namespace:    "another-test-namespace",
			Microservice: "test-microservice",
			Name:         "test-topic",
			EntityType:   EntityTypeTopic,
		})
		assert.NoError(t, err)

		stats, err := serviceDao.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 3, len(*stats))

		err = serviceDao.DeleteEntityRequestsStatByNamespace(ctx, "test-namespace")
		assert.NoError(t, err)

		stats, err = serviceDao.GetAllEntityRequestsStat(ctx)
		assert.NoError(t, err)
		assert.Equal(t, 1, len(*stats))
	})
}

func TestDaoImpl_GetMonitoringVhosts(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		serviceDao := NewDao(baseDao)

		_, err := serviceDao.GetMonitoringVhosts(ctx)
		assert.NoError(t, err)
	})
}

func TestDaoImpl_GetMonitoringTopics(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		serviceDao := NewDao(baseDao)

		_, err := serviceDao.GetMonitoringTopics(ctx)
		assert.NoError(t, err)
	})
}
