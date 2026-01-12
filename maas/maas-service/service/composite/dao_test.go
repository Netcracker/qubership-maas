package composite

import (
	"context"
	"testing"

	"github.com/netcracker/qubership-maas/dao"
	"github.com/stretchr/testify/assert"
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
