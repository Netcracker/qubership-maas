package domain

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestAPI(t *testing.T) {
	dao.WithSharedDao(t, func(base *dao.BaseDaoImpl) {
		ctx := context.Background()
		service := NewBGDomainService(NewBGDomainDao(base))

		{
			list, err := service.List(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, list)
			assert.Equal(t, 0, len(list))
		}

		assert.NoError(t, service.Bind(ctx, "a", "b", "c"))

		domain, err := service.FindByNamespace(ctx, "b")
		assert.NoError(t, err)
		assert.Equal(t, &BGNamespaces{"a", "b", "c"}, domain)

		{
			found, err := service.FindByNamespace(ctx, "other")
			assert.NoError(t, err)
			assert.Nil(t, found)
		}

		{
			list, err := service.List(ctx)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(list))
			assert.Equal(t, BGNamespaces{"a", "b", "c"}, (list)[0])
		}

		{
			actual, err := service.GetCurrentBgStateByNamespace(ctx, "a")
			assert.NoError(t, err)
			assert.Nil(t, actual)

			state := BGState{
				ControllerNamespace: "bg-controller",
				Origin:              &BGNamespace{Name: "origin", State: "active", Version: "v1"},
				Peer:                &BGNamespace{Name: "peer", State: "idle", Version: ""},
				UpdateTime:          time.Now(),
			}

			err = service.InsertBgState(ctx, state)
			assert.ErrorIs(t, err, msg.BadRequest)

			// fix origin, but peer stay incorrect
			state.Origin.Name = "a"
			err = service.InsertBgState(ctx, state)
			assert.ErrorIs(t, err, msg.BadRequest)

			state.Peer.Name = "b"
			err = service.InsertBgState(ctx, state)
			assert.NoError(t, err)

			actual, err = service.GetCurrentBgStateByNamespace(ctx, "a")
			assert.NoError(t, err)
			assert.Equal(t, model.Version("v1"), actual.Origin.Version)

			// test housekeeping
			state.UpdateTime = time.Now().Add(1 * time.Second)
			state.Peer.Version = "v2"
			assert.NoError(t, service.InsertBgState(ctx, state))
			state.UpdateTime = time.Now().Add(2 * time.Second)
			state.Peer.Version = "v3"
			assert.NoError(t, service.InsertBgState(ctx, state))
			count, err := service.dao.(*BGDomainDaoImpl).DeleteObsoleteStates(ctx, 2)
			assert.NoError(t, err)
			assert.Equal(t, int64(1), count)

			// test that correct state version remains in db
			actual, err = service.GetCurrentBgStateByNamespace(ctx, "a")
			assert.NoError(t, err)
			assert.Equal(t, model.Version("v3"), actual.Peer.Version)
		}

		domain, err = service.Unbind(ctx, "a")
		assert.NoError(t, err)
		assert.Equal(t, &BGNamespaces{"a", "b", "c"}, domain)

		_, err = service.Unbind(ctx, "a")
		assert.Error(t, err)
	})
}

func TestDaoMigration(t *testing.T) {
	// first BG2 implementation didn't have value for controller namespace, we need to update it in new versions
	testharness.WithSharedTestDatabase(t, func(tdb *testharness.TestDatabase) {

		base := dao.New(&db.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 10,
			DrMode:   dr.Active,
		})
		defer base.Close()

		ctx := context.Background()
		service := NewBGDomainService(NewBGDomainDao(base))

		assert.NoError(t, service.Bind(ctx, "a", "b", ""))

		// re-enterability test
		assert.NoError(t, service.Bind(ctx, "a", "b", ""))

		found1, err := service.FindByNamespace(ctx, "a")
		assert.NoError(t, err)
		assert.Equal(t, &BGNamespaces{"a", "b", ""}, found1)

		assert.NoError(t, service.UpdateController(ctx, "a", "c"))
		found2, err := service.FindByNamespace(ctx, "a")
		assert.NoError(t, err)
		assert.Equal(t, &BGNamespaces{"a", "b", "c"}, found2)

		// no error with the same controller value
		assert.NoError(t, service.UpdateController(ctx, "a", "c"), msg.BadRequest)

		// conflict trying change controller
		assert.ErrorIs(t, service.UpdateController(ctx, "a", "g"), msg.BadRequest)
	})
}
