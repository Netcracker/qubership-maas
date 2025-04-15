package postdeploy

import (
	"context"
	"github.com/golang/mock/gomock"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/stretchr/testify/assert"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAccounts_NoOp(t *testing.T) {
	ctx := context.Background()
	createManagerAccount(ctx, nil)
}

func TestAccounts_ManagerCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	authService := NewMockAuthService(mockCtrl)
	gomock.InOrder(
		authService.EXPECT().GetAccountByUsername(gomock.Eq(ctx), gomock.Eq("scott")).
			Return(nil, nil).
			Times(1),
		authService.EXPECT().CreateNewManager(gomock.Eq(ctx), gomock.Eq(
			&model.ManagerAccountDto{
				Username: "scott",
				Password: "tiger",
			})).
			Return(nil, nil).
			Times(1),

		// test account update
		authService.EXPECT().GetAccountByUsername(gomock.Eq(ctx), gomock.Eq("scott")).
			Return(&model.Account{
				Username: "scott",
				Password: "tiger",
			}, nil).
			Times(1),
		authService.EXPECT().UpdateUserPassword(gomock.Eq(ctx), gomock.Eq("scott"), gomock.Eq(utils.SecretString("tiger"))).
			Return(nil).
			Times(1),
	)

	// change root folder to temp with mock content
	EtcMaaSRoot = filepath.Join(os.TempDir())
	withAccountFile(t, "manager-username", "scott", func() {
		withAccountFile(t, "manager-password", "tiger", func() {
			err := createManagerAccount(ctx, authService)
			assert.NoError(t, err)

			// update account
			err = createManagerAccount(ctx, authService)
			assert.NoError(t, err)
		})
	})
}

func TestAccounts_DeployerClientCreateAndUpdate(t *testing.T) {
	ctx := context.Background()
	mockCtrl := gomock.NewController(t)

	authService := NewMockAuthService(mockCtrl)
	gomock.InOrder(
		authService.EXPECT().GetAccountByUsername(gomock.Eq(ctx), gomock.Eq("scott")).
			Return(nil, nil).
			Times(1),
		authService.EXPECT().CreateUserAccount(gomock.Eq(ctx), gomock.Eq(
			&model.ClientAccountDto{
				Username:  "scott",
				Password:  "tiger",
				Namespace: model.ManagerAccountNamespace,
				Roles:     []model.RoleName{model.ManagerRole, model.AgentRole},
			})).
			Return(true, nil).
			Times(1),

		// test account update
		authService.EXPECT().GetAccountByUsername(gomock.Eq(ctx), gomock.Eq("scott")).
			Return(&model.Account{
				Username: "scott",
				Password: "tiger",
			}, nil).
			Times(1),
		authService.EXPECT().UpdateUserPassword(gomock.Eq(ctx), gomock.Eq("scott"), gomock.Eq(utils.SecretString("tiger"))).
			Return(nil).
			Times(1),
	)

	// change root folder to temp with mock content
	EtcMaaSRoot = filepath.Join(os.TempDir())
	withAccountFile(t, "deployer-username", "scott", func() {
		withAccountFile(t, "deployer-password", "tiger", func() {
			err := createDeployerAccount(ctx, authService)
			assert.NoError(t, err)

			// update account
			err = createDeployerAccount(ctx, authService)
			assert.NoError(t, err)
		})
	})
}

func withAccountFile(t *testing.T, name string, content string, callback func()) {
	accountsDir := filepath.Join(EtcMaaSRoot, "maas-accounts")
	if _, err := os.Stat(accountsDir); err != nil && os.IsNotExist(err) {
		err = os.Mkdir(accountsDir, 0755)
		assert.NoError(t, err)
	}
	path := filepath.Join(accountsDir, name)
	_ = os.Remove(path)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0666)
	assert.NoError(t, err)
	_, err = io.WriteString(file, content)
	assert.NoError(t, err)
	defer os.Remove(path)

	callback()

}
