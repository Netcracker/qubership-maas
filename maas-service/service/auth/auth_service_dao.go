package auth

import (
	"context"
	"errors"
	"fmt"
	"github.com/netcracker/qubership-maas/dao"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
)

type AuthServiceImpl struct {
	dao                AuthDao
	bgDomainService    domain.BGDomainService
	compositeRegistrar CompositeRegistrar
}

type AuthDao interface {
	GetAccountByUsername(context.Context, string) (*model.Account, error)
	DeleteAccount(context.Context, string) error
	FindAccountByRoleAndNamespace(context.Context, model.RoleName, string) ([]model.Account, error)
	SaveAccount(context.Context, *model.Account) error
	GetAllAccounts(context.Context) (*[]model.Account, error)
	UpdateUserPassword(ctx context.Context, username string, password utils.SecretString, salt utils.SecretString) error
}

type AuthDaoImpl struct {
	base dao.BaseDao
}

func NewAuthDao(dao dao.BaseDao) *AuthDaoImpl {
	return &AuthDaoImpl{base: dao}
}

func (d *AuthDaoImpl) GetAccountByUsername(ctx context.Context, username string) (*model.Account, error) {
	log.InfoC(ctx, "GetAccountByUsername for username: %v", username)
	if username == "" {
		return nil, utils.LogError(log, ctx, "username can't be empty: %w", msg.BadRequest)
	}

	obj := model.Account{}
	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Where("LOWER(username) = LOWER(?)", username).First(&obj).Error
	})

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}

		return nil, utils.LogError(log, ctx, "error getting account: %w", err)
	}

	return &obj, nil
}

func (d *AuthDaoImpl) FindAccountByRoleAndNamespace(ctx context.Context, managerRole model.RoleName, namespace string) ([]model.Account, error) {
	log.InfoC(ctx, "Find accounts by role: %v, namespace: %v", managerRole, namespace)
	var accounts []model.Account

	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Model(model.Account{}).
			Where("roles ? ?", gorm.Expr(`?`), managerRole).
			Where("namespace = ?", namespace).
			Find(&accounts).Error
	})

	if err != nil {
		return nil, utils.LogError(log, ctx, "error count accounts by role `%s`: %w", managerRole, err)
	}

	log.InfoC(ctx, "Found accounts: %v", accounts)
	return accounts, nil
}

func (d *AuthDaoImpl) GetAllAccounts(ctx context.Context) (*[]model.Account, error) {
	log.InfoC(ctx, "GetAllAccounts dao")
	obj := new([]model.Account)

	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Find(obj).Error
	})
	if err != nil {
		return nil, utils.LogError(log, ctx, "error list of accounts: %w", err)
	}

	return obj, nil
}

func (d *AuthDaoImpl) SaveAccount(ctx context.Context, account *model.Account) error {
	log.InfoC(ctx, "Create account: '%+v'", account)

	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		return cnn.Create(account).Error
	})
	if err != nil {
		return utils.LogError(log, ctx, "error create account: %w", err)
	}

	return nil
}

func (d *AuthDaoImpl) DeleteAccount(ctx context.Context, username string) error {
	log.InfoC(ctx, "Delete account by username: `%v'", username)
	if username == "" {
		return utils.LogError(log, ctx, "username can't be empty: %w", msg.BadRequest)
	}

	err := d.base.UsingDb(ctx, func(cnn *gorm.DB) error {
		rs := cnn.Where("username=?", username).Delete(&model.Account{})
		if rs.Error != nil {
			return rs.Error
		}

		if rs.RowsAffected != 1 {
			return fmt.Errorf("not account found for `%v': %w", username, msg.NotFound)
		}

		return nil
	})
	if err != nil {
		return utils.LogError(log, ctx, "error delete account: %w", err)
	}

	return nil
}

func (d *AuthDaoImpl) UpdateUserPassword(ctx context.Context, username string, password utils.SecretString, salt utils.SecretString) error {
	log.InfoC(ctx, "UpdateUserPassword dao: '%s'", username)

	err := d.base.WithTx(ctx, func(ctx context.Context, cnn *gorm.DB) error {
		updates := cnn.Model(&model.Account{}).Where("username = ?", username).Updates(model.Account{
			Salt:     salt,
			Password: password,
		})
		if updates.RowsAffected == 0 {
			return fmt.Errorf("user with name '%s' not found: %w", username, msg.NotFound)
		}
		return updates.Error
	})

	if err != nil {
		log.ErrorC(ctx, "Error UpdateUserPassword: %v", err.Error())
		return err
	}

	return nil
}
