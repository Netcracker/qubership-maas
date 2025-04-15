package postdeploy

import (
	"context"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/utils"
	"path/filepath"
)

//go:generate mockgen -source=accounts.go -destination=accounts_mock.go -package=postdeploy
type AuthService interface {
	CreateNewManager(ctx context.Context, account *model.ManagerAccountDto) (*model.ManagerAccountDto, error)
	CreateUserAccount(ctx context.Context, accountRequest *model.ClientAccountDto) (isNew bool, err error)
	GetAccountByUsername(ctx context.Context, username string) (*model.Account, error)
	UpdateUserPassword(ctx context.Context, username string, password utils.SecretString) error
}

func createAccounts(ctx context.Context, account string, auth AuthService, finalizer func(username, password string) error) error {
	usernamePath := filepath.Join(EtcMaaSRoot, "maas-accounts", account+"-username")
	passwordPath := filepath.Join(EtcMaaSRoot, "maas-accounts", account+"-password")
	return readFile(ctx, usernamePath, func(usernameBytes []byte) error {
		return readFile(ctx, passwordPath, func(passwordBytes []byte) error {
			username := string(usernameBytes)
			password := string(passwordBytes)
			log.InfoC(ctx, "Setup account: `%v`", username)
			account, err := auth.GetAccountByUsername(ctx, username)
			if err != nil {
				return utils.LogError(log, ctx, "error get account by `%s`: %w", username, err)
			}

			if account != nil {
				log.InfoC(ctx, "Update account password for: `%s`", username)
				err = auth.UpdateUserPassword(ctx, username, utils.SecretString(password))
				if err != nil {
					return utils.LogError(log, ctx, "error update account `%s` password: %w", username, err)
				}

				log.InfoC(ctx, "Password updated for: `%s`", username)
				return nil
			}

			return finalizer(username, password)
		})
	})
}

func createManagerAccount(ctx context.Context, auth AuthService) error {
	return createAccounts(ctx, "manager", auth, func(username, password string) error {
		_, err := auth.CreateNewManager(ctx, &model.ManagerAccountDto{
			Username: username,
			Password: utils.SecretString(password),
		})
		if err != nil {
			return utils.LogError(log, ctx, "couldn't create manager account while in createManagerAccount: %w", err)
		}

		log.InfoC(ctx, "First manager account was created successfully during bootstrap")
		return nil
	})
}

func createDeployerAccount(ctx context.Context, auth AuthService) error {
	return createAccounts(ctx, "deployer", auth, func(username, password string) error {
		_, err := auth.CreateUserAccount(ctx, &model.ClientAccountDto{
			Username:  username,
			Password:  utils.SecretString(password),
			Namespace: model.ManagerAccountNamespace,
			Roles:     []model.RoleName{model.ManagerRole, model.AgentRole},
		})

		if err != nil {
			return utils.LogError(log, ctx, "error create deployer account: %w", err)
		}

		log.InfoC(ctx, "Deployer client account was created successfully during bootstrap")
		return nil
	})
}
