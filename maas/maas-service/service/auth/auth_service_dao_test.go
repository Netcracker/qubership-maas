package auth

import (
	"context"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAuthDaoImpl_API(t *testing.T) {
	user := "test-user"
	ctx := context.Background()
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		dao := NewAuthDao(baseDao)

		accs, err := dao.GetAllAccounts(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, accs)
		assert.Equal(t, 0, len(*accs))

		{
			account := &model.Account{
				Username:  user,
				Roles:     []model.RoleName{model.AgentRole},
				Namespace: "first",
			}
			err = dao.SaveAccount(ctx, account)
			assert.NoError(t, err)

			err = dao.SaveAccount(ctx, account)
			assert.Error(t, err)
		}

		{
			account := &model.Account{
				Username:  "manager",
				Roles:     []model.RoleName{model.ManagerRole},
				Namespace: "first",
			}
			err = dao.SaveAccount(ctx, account)
			assert.NoError(t, err)
		}

		{
			accounts, err := dao.FindAccountByRoleAndNamespace(ctx, model.ManagerRole, "first")
			assert.NoError(t, err)
			assert.Equal(t, 1, len(accounts))
		}

		{
			account, err := dao.GetAccountByUsername(ctx, user)
			assert.NoError(t, err)
			assert.Equal(t, user, account.Username)
			assert.Equal(t, model.AgentRole, account.Roles[0])
		}

		err = dao.UpdateUserPassword(ctx, "non-existing", "", "")
		assert.ErrorIs(t, err, msg.NotFound)

		err = dao.UpdateUserPassword(ctx, user, "tiger", "abc")
		assert.NoError(t, err)
	})
}
