package auth

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"github.com/google/uuid"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"golang.org/x/crypto/pbkdf2"
	"maas/maas-service/dao"
	"maas/maas-service/model"
	"maas/maas-service/msg"
	"maas/maas-service/service/bg2/domain"
	"maas/maas-service/service/composite"
	"maas/maas-service/utils"
	"reflect"
	"strings"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("auth")
}

const (
	PBKDF2_ITERATIONS = 4096
	PBKDF2_KEY_LENG   = 64
)

//go:generate mockgen -source=auth_service.go -destination=mock/auth_service.go
type AuthService interface {
	CreateUserAccount(ctx context.Context, accountRequest *model.ClientAccountDto) (isNew bool, err error)
	DeleteUserAccount(ctx context.Context, username string) error
	IsFirstAccountManager(ctx context.Context) (bool, error)
	CreateNewManager(ctx context.Context, accountRequest *model.ManagerAccountDto) (*model.ManagerAccountDto, error)
	IsAccessGranted(ctx context.Context, username string, password utils.SecretString, namespace string, role []model.RoleName) (*model.Account, error)
	GetAllAccounts(ctx context.Context) (*[]model.Account, error)
	UpdateUserPassword(ctx context.Context, username string, password utils.SecretString) error
	GetAccountByUsername(ctx context.Context, username string) (*model.Account, error)
	GetAllowedNamespaces(ctx context.Context, namespace string) ([]string, error)

	CheckSecurityForBoundNamespaces(ctx context.Context, namespace string, classifier *model.Classifier) error
	CheckSecurityForSingleNamespace(ctx context.Context, namespace string, classifier *model.Classifier) error
}

type CompositeRegistrar interface {
	GetByNamespace(ctx context.Context, namespace string) (*composite.CompositeRegistration, error)
}

func NewAuthService(dao AuthDao, compositeRegistrar CompositeRegistrar, bgDomainService domain.BGDomainService) AuthService {
	return &AuthServiceImpl{
		dao:                dao,
		compositeRegistrar: compositeRegistrar,
		bgDomainService:    bgDomainService,
	}
}

func (s *AuthServiceImpl) CreateUserAccount(ctx context.Context, accountRequest *model.ClientAccountDto) (bool, error) {
	log.InfoC(ctx, "Create account: %v", accountRequest)
	return dao.WithTransactionContextValue(ctx, func(ctx context.Context) (bool, error) {
		account := &model.Account{
			Username:  accountRequest.Username,
			Password:  accountRequest.Password,
			Roles:     accountRequest.Roles,
			Namespace: accountRequest.Namespace,
		}

		found, err := s.dao.GetAccountByUsername(ctx, account.Username)
		if err != nil {
			return false, utils.LogError(log, ctx, "error getting account by username `%v': %w", account.Username, err)
		}

		if found != nil &&
			account.Username == found.Username &&
			encryptPassword(account.Password, found.Salt) == found.Password &&
			account.Namespace == found.Namespace &&
			reflect.DeepEqual(account.Roles, found.Roles) {

			return false, nil
		}

		account, err = s.createAccount(ctx, account)
		if err != nil {
			return false, err
		}

		return true, nil
	})
}

func (s *AuthServiceImpl) DeleteUserAccount(ctx context.Context, username string) error {
	return s.dao.DeleteAccount(ctx, username)
}

func (s *AuthServiceImpl) IsFirstAccountManager(ctx context.Context) (bool, error) {
	accounts, err := s.dao.FindAccountByRoleAndNamespace(ctx, model.ManagerRole, model.ManagerAccountNamespace)
	if err != nil {
		return false, err
	}

	if accounts == nil || len(accounts) == 0 {
		log.InfoC(ctx, "Manager account not found")
		return true, nil
	} else {
		log.InfoC(ctx, "Manager account already registered: %+v", accounts)
		return false, nil
	}
}

func (s *AuthServiceImpl) CreateNewManager(ctx context.Context, accountRequest *model.ManagerAccountDto) (*model.ManagerAccountDto, error) {
	return dao.WithTransactionContextValue(ctx, func(ctx context.Context) (*model.ManagerAccountDto, error) {
		log.InfoC(ctx, "Create new manager account: %v", accountRequest)
		if account, err := s.dao.GetAccountByUsername(ctx, accountRequest.Username); err != nil {
			return nil, utils.LogError(log, ctx, "error during search account by username. %w", err)
		} else if account != nil {
			return nil, utils.LogError(log, ctx, "account with username `%v' already exists: %w", accountRequest.Username, msg.Conflict)
		}
		log.DebugC(ctx, "Manager account with username %s does not exist", accountRequest.Username)
		managerAccount, err := s.createManagerAccount(ctx, accountRequest)
		if err != nil {
			return nil, utils.LogError(log, ctx, "error create manager account: %w", err)
		}
		return managerAccount, nil
	})
}

func (s *AuthServiceImpl) createManagerAccount(ctx context.Context, accountRequest *model.ManagerAccountDto) (*model.ManagerAccountDto, error) {
	account := &model.Account{Username: accountRequest.Username, Password: accountRequest.Password, Roles: []model.RoleName{model.ManagerRole}, Namespace: model.ManagerAccountNamespace}
	account, err := s.createAccount(ctx, account)
	if err == nil {
		accountRequest.Username = account.Username
		accountRequest.Password = account.Password
		return accountRequest, nil
	}
	return nil, err
}

func (s *AuthServiceImpl) createAccount(ctx context.Context, account *model.Account) (*model.Account, error) {
	// TODO validation??
	if account.Username == "" {
		return nil, utils.LogError(log, ctx, "account username can't be empty: %w", msg.BadRequest)
	}
	if account.Password == "" {
		return nil, utils.LogError(log, ctx, "account password can't be empty: %w", msg.BadRequest)
	}
	if account.Namespace == "" {
		return nil, utils.LogError(log, ctx, "account should be bound to namespace: %w", msg.BadRequest)
	}
	if account.Roles == nil {
		return nil, utils.LogError(log, ctx, "account should have at least one role: %w", msg.BadRequest)
	}
	salt := generateSalt()
	encryptedPassword := encryptPassword(account.Password, salt)
	accountForSave := &model.Account{Username: account.Username, Salt: salt, Roles: account.Roles, Password: encryptedPassword, Namespace: account.Namespace}
	if err := s.dao.SaveAccount(ctx, accountForSave); err != nil {
		return nil, err
	}
	return account, nil
}

func (s *AuthServiceImpl) IsAccessGranted(ctx context.Context, username string, password utils.SecretString, namespace string, roles []model.RoleName) (*model.Account, error) {
	log.InfoC(ctx, "Checking access for account with username '%v', roles '%v'", username, roles)
	account, err := s.dao.GetAccountByUsername(ctx, username)
	if err != nil {
		return nil, utils.LogError(log, ctx, "failed to find account for username '%v': %w", username, err)
	}
	if account == nil {
		return nil, utils.LogError(log, ctx, "account with username `%s' does not exist: %w", username, msg.AuthError)
	}
	actualPass := encryptPassword(password, account.Salt)
	if actualPass != account.Password {
		return nil, utils.LogError(log, ctx, "password verification failed for client with username `%s': %w", username, msg.AuthError)
	}

	if !account.HasRoles(model.ManagerRole, model.BgOperatorRole) {
		if namespace == "" {
			return nil, utils.LogError(log, ctx, "you're trying to authenticate with account `%s' that doesn't have 'manager' role (existing roles: %+v), therefore namespace should be mandatory specified: %w", username, roles, msg.BadRequest)
		}

		if !account.HasAccessToNamespace(namespace) {
			namespaces, err := s.bgDomainService.FindByNamespace(ctx, account.Namespace)
			if err != nil {
				return nil, utils.LogError(log, ctx, "can not get bg domain for user '%s': %w", account.Username, msg.AuthError)
			}
			if namespaces == nil {
				return nil, utils.LogError(log, ctx, "user '%v' has no access to namespace '%v': %w", username, namespace, msg.AuthError)
			}
			if !namespaces.IsMember(namespace) {
				return nil, utils.LogError(log, ctx, "user '%v' has no access to namespace '%v', available namespaces '%v': %w", username, namespace, *namespaces, msg.AuthError)
			}
		}
	}
	if !account.HasRoles(roles...) {
		return nil, utils.LogError(log, ctx, "user `%s' doesn't have enough roles (%+v) for requested operation: %w", username, roles, msg.AuthError)
	}
	return account, nil
}

func (s *AuthServiceImpl) GetAllAccounts(ctx context.Context) (*[]model.Account, error) {
	if accounts, err := s.dao.GetAllAccounts(ctx); err == nil {
		return accounts, nil
	} else {
		return nil, utils.LogError(log, ctx, "error during search all accounts: %w", err)
	}
}

func (s *AuthServiceImpl) UpdateUserPassword(ctx context.Context, username string, password utils.SecretString) error {
	return dao.WithTransactionContext(ctx, func(ctx context.Context) error {
		if password == "" {
			return utils.LogError(log, ctx, "password can't be empty: %w", msg.BadRequest)
		}
		salt := generateSalt()
		if err := s.dao.UpdateUserPassword(ctx, username, encryptPassword(password, salt), salt); err != nil {
			return utils.LogError(log, ctx, "error change password for `%v': %w", username, err)
		}
		return nil
	})
}

func (s *AuthServiceImpl) GetAccountByUsername(ctx context.Context, username string) (*model.Account, error) {
	return s.dao.GetAccountByUsername(ctx, username)
}

func (s *AuthServiceImpl) GetAllowedNamespaces(ctx context.Context, namespace string) ([]string, error) {
	compositeRegistration, err := s.compositeRegistrar.GetByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "error search composite registration: %w", err)
	}

	var allowedNamespaces []string
	if compositeRegistration != nil {
		allowedNamespaces = compositeRegistration.Namespaces
	}

	bgNamespaces, err := s.bgDomainService.FindByNamespace(ctx, namespace)
	if err != nil {
		return nil, utils.LogError(log, ctx, "can not get bg namespaces: %w", err)
	}
	if bgNamespaces != nil {
		allowedNamespaces = append(allowedNamespaces, bgNamespaces.Origin, bgNamespaces.Peer)
	} else {
		allowedNamespaces = append(allowedNamespaces, namespace)
	}
	return allowedNamespaces, nil
}

func (s *AuthServiceImpl) CheckSecurityForBoundNamespaces(ctx context.Context, namespace string, classifier *model.Classifier) error {
	namespaces, err := s.GetAllowedNamespaces(ctx, namespace)
	if err != nil {
		return err
	}
	if err = classifier.CheckAllowedNamespaces(namespaces...); err != nil {
		return utils.LogError(log, ctx, "access denied: %w", err)
	}
	return nil
}

func (s *AuthServiceImpl) CheckSecurityForSingleNamespace(ctx context.Context, namespace string, classifier *model.Classifier) error {
	if err := classifier.CheckAllowedNamespaces(namespace); err != nil {
		return utils.LogError(log, ctx, "access denied: %w", err)
	}
	return nil
}

func generateSalt() utils.SecretString {
	return utils.SecretString(strings.ReplaceAll(uuid.New().String(), "-", "")[:10])
}

func encryptPassword(password utils.SecretString, salt utils.SecretString) utils.SecretString {
	hash := pbkdf2.Key([]byte(password), []byte(salt), PBKDF2_ITERATIONS, PBKDF2_KEY_LENG, sha1.New)
	return utils.SecretString(
		base64.StdEncoding.EncodeToString(hash),
	)
}
