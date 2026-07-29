package composite

import (
	"context"
	"math"
	"testing"

	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestPGRegistrationDao_API(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		{
			list, err := dao.List(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, list)
			assert.Equal(t, 0, len(list))
		}

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))
		// the same second time
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))

		// try to insert incorrect structure
		assert.Error(t, dao.Upsert(ctx, &CompositeRegistration{Id: "b", Namespaces: []string{"c"}}))

		// add other composite with non interleaving namespaces
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "f", Namespaces: []string{"f", "e", "d"}}))

		list, err := dao.List(ctx)
		assert.NoError(t, err)
		assert.Equal(t, []CompositeRegistration{
			{Id: "a", Namespaces: []string{"a", "b"}},
			{Id: "f", Namespaces: []string{"d", "e", "f"}},
		}, list)

		// update registration with new member "c"
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b", "c"}}))

		// update registration by removing "b"
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "c"}}))

		{
			registration, err := dao.GetByBaseline(ctx, "a")
			assert.NoError(t, err)
			assert.Equal(t, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "c"}}, registration)
		}

		assert.NoError(t, dao.DeleteByBaseline(ctx, "a"))

		{
			registration, err := dao.GetByBaseline(ctx, "a")
			assert.NoError(t, err)
			assert.Nil(t, registration)
		}

		{
			registration, err := dao.GetByBaseline(ctx, "f")
			assert.NoError(t, err)
			assert.Equal(t, &CompositeRegistration{Id: "f", Namespaces: []string{"d", "e", "f"}}, registration)
		}
	})
}

func TestPGRegistrationDao_Upsert_Conflicts(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))
		assert.Error(t, dao.Upsert(ctx, &CompositeRegistration{Id: "b", Namespaces: []string{"b", "c"}}))
	})
}

func TestPGRegistrationDao_Upsert_WrongModifyIndex(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(100))}))
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))
		assert.ErrorContains(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(100))}),
			"new modify index '100' cannot be less than the current index '200'",
		)
	})
}

func TestPGRegistrationDao_Upsert_ModifyIndexIsBadRequest(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))

		err := dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(199))})
		// client must get 400, not 500
		assert.ErrorIs(t, err, msg.BadRequest)
	})
}

func TestPGRegistrationDao_Upsert_RejectedModifyIndexKeepsRegistrationIntact(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))
		assert.Error(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b", "c"}, ModifyIndex: ptr(uint64(100))}))

		// rejected update must not be applied even partially
		registration, err := dao.GetByBaseline(ctx, "a")
		assert.NoError(t, err)
		assert.Equal(t, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}, registration)
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: 200}}, readModifyIndexes(t, baseDao))
	})
}

func TestPGRegistrationDao_Upsert_ModifyIndexStoredOnBaselineNamespaceOnly(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b", "c"}, ModifyIndex: ptr(uint64(100))}))
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: 100}}, readModifyIndexes(t, baseDao))

		// composite_properties row is removed together with the registration it references
		assert.NoError(t, dao.DeleteByBaseline(ctx, "a"))
		assert.Empty(t, readModifyIndexes(t, baseDao))
	})
}

func TestPGRegistrationDao_Upsert_ModifyIndexIsPerComposite(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))
		// low index of an independent composite must not be compared against composite 'a'
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "d", Namespaces: []string{"d", "e"}, ModifyIndex: ptr(uint64(1))}))

		assert.Equal(t, []modifyIndexRow{
			{Namespace: "a", ModifyIndex: 200},
			{Namespace: "d", ModifyIndex: 1},
		}, readModifyIndexes(t, baseDao))
	})
}

func TestPGRegistrationDao_Upsert_ModifyIndexOnUntrackedComposite(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		// composite registered before modify index tracking existed has no stored index
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))
		assert.Empty(t, readModifyIndexes(t, baseDao))

		// missing index is treated as 0, so any index is accepted
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(0))}))
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: 0}}, readModifyIndexes(t, baseDao))

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(5))}))
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: 5}}, readModifyIndexes(t, baseDao))
	})
}

func TestPGRegistrationDao_Upsert_OmittedModifyIndexDropsStoredIndex(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(200))}))

		// upsert without modify index skips the check and drops the stored index along with the old rows
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))
		assert.Empty(t, readModifyIndexes(t, baseDao))

		// so an index lower than the previously stored one is accepted again
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(1))}))
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: 1}}, readModifyIndexes(t, baseDao))
	})
}

func TestPGRegistrationDao_Upsert_MaxModifyIndex(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		// NUMERIC(20) must hold the whole uint64 range without truncation
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(math.MaxUint64))}))
		assert.Equal(t, []modifyIndexRow{{Namespace: "a", ModifyIndex: math.MaxUint64}}, readModifyIndexes(t, baseDao))

		assert.ErrorContains(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}, ModifyIndex: ptr(uint64(math.MaxUint64 - 1))}),
			"new modify index '18446744073709551614' cannot be less than the current index '18446744073709551615'",
		)
	})
}

type modifyIndexRow struct {
	Namespace   string
	ModifyIndex uint64
}

func readModifyIndexes(t *testing.T, baseDao *dao.BaseDaoImpl) []modifyIndexRow {
	rows := make([]modifyIndexRow, 0)
	assert.NoError(t, baseDao.UsingDb(context.Background(), func(cnn *gorm.DB) error {
		return cnn.
			Table("composite_properties c").
			Select("n.namespace as namespace, c.modify_index as modify_index").
			Joins("JOIN composite_namespaces_v2 n ON n.id = c.composite_namespace_id").
			Order("n.namespace").
			Scan(&rows).Error
	}))
	return rows
}

func TestPGRegistrationDao_FindByNamespace(t *testing.T) {
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewPGRegistrationDao(baseDao)

		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "a", Namespaces: []string{"a", "b"}}))
		assert.NoError(t, dao.Upsert(ctx, &CompositeRegistration{Id: "d", Namespaces: []string{"f", "d", "c"}}))

		{
			// find composite registration by one of its member
			registration, err := dao.GetByNamespace(ctx, "c")
			assert.NoError(t, err)
			assert.Equal(t, &CompositeRegistration{Id: "d", Namespaces: []string{"c", "d", "f"}}, registration)
		}

		{
			// find composite registration by one of its member
			registration, err := dao.GetByNamespace(ctx, "non-existing")
			assert.NoError(t, err)
			assert.Nil(t, registration)
		}
	})
}

func ptr[T any](v T) *T {
	return &v
}
