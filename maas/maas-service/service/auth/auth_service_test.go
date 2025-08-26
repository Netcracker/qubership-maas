package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/netcracker/qubership-maas/dao"
	dbc "github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/kubernetes/oidc"
	"github.com/netcracker/qubership-maas/model"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/service/bg2/domain"
	"github.com/netcracker/qubership-maas/service/composite"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/stretchr/testify/assert"
)

const (
	managerName     = "test-manager"
	managerPassword = "test-manager-password"
)

func TestAuthServiceImpl_API(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx := context.Background()

		dao := NewAuthDao(baseDao)
		authService := NewAuthService(dao, nil, nil)

		account := model.ClientAccountDto{
			Username:  "scott",
			Password:  "tiger",
			Namespace: "core-dev",
			Roles:     []model.RoleName{model.AgentRole},
		}

		// create account
		{ // no username
			a := account
			a.Username = ""
			_, err := authService.CreateUserAccount(ctx, &a)
			assert.ErrorIs(t, err, msg.BadRequest)
		}
		{ // no password
			a := account
			a.Password = ""
			_, err := authService.CreateUserAccount(ctx, &a)
			assert.ErrorIs(t, err, msg.BadRequest)
		}
		{ // missed namespace
			a := account
			a.Namespace = ""
			_, err := authService.CreateUserAccount(ctx, &a)
			assert.ErrorIs(t, err, msg.BadRequest)
		}
		{ // missed roles
			a := account
			a.Roles = nil
			_, err := authService.CreateUserAccount(ctx, &a)
			assert.ErrorIs(t, err, msg.BadRequest)
		}
		{ // successful account creation
			isNew, err := authService.CreateUserAccount(ctx, &account)
			assert.NoError(t, err)
			assert.True(t, isNew)
		}
		{
			isNew, err := authService.CreateUserAccount(ctx, &account)
			assert.NoError(t, err)
			assert.False(t, isNew)
		}
		{
			accounts, err := authService.GetAllAccounts(ctx)
			assert.NoError(t, err)
			assert.Len(t, *accounts, 1)
		}

		// delete account
		{
			err := authService.DeleteUserAccount(ctx, "")
			assert.ErrorIs(t, err, msg.BadRequest)
		}
		{
			err := authService.DeleteUserAccount(ctx, account.Username)
			assert.NoError(t, err)
		}
		{
			err := authService.DeleteUserAccount(ctx, account.Username)
			assert.ErrorIs(t, err, msg.NotFound)
		}

	})
}

func TestAuthServiceImpl_UpdateUserPassword(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		dao := NewAuthDao(baseDao)
		authService := NewAuthService(dao, nil, nil)

		{
			managerExists, err := authService.IsFirstAccountManager(ctx)
			assert.NoError(t, err)
			assert.True(t, managerExists)
		}

		_, err := authService.CreateNewManager(ctx, &model.ManagerAccountDto{
			Username: managerName,
			Password: managerPassword,
		})
		assert.NoError(t, err)
		accountBeforeUpdate, err := authService.GetAccountByUsername(ctx, managerName)
		assert.NoError(t, err)

		err = authService.UpdateUserPassword(ctx, "not_existed_user", "new-"+managerPassword)
		assert.True(t, strings.Contains(err.Error(), "user with name 'not_existed_user' not found"))

		err = authService.UpdateUserPassword(ctx, managerName, "new-"+managerPassword)
		assert.NoError(t, err)
		accountAfterUpdate, err := authService.GetAccountByUsername(ctx, managerName)
		assert.NoError(t, err)
		fmt.Printf("%+v", accountAfterUpdate)

		{
			managerExists, err := authService.IsFirstAccountManager(ctx)
			assert.NoError(t, err)
			assert.False(t, managerExists)
		}

		assert.NotEqual(t, managerPassword, accountBeforeUpdate.Password)
		assert.NotEqual(t, "new-"+managerPassword, accountAfterUpdate.Password)

		assert.NotEqual(t, accountBeforeUpdate.Password, accountAfterUpdate.Password)
		assert.NotEqual(t, accountBeforeUpdate.Salt, accountAfterUpdate.Salt)
		assert.Equal(t, accountBeforeUpdate.Username, accountAfterUpdate.Username)
		assert.Equal(t, accountBeforeUpdate.Roles, accountAfterUpdate.Roles)
		assert.Equal(t, accountBeforeUpdate.Namespace, accountAfterUpdate.Namespace)
	})
}

func TestReplication(t *testing.T) {
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()

	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		// to have better control over database connectivity start tcpproxy and connect to database over it
		localAddr := "localhost:5434"
		proxy := testharness.NewTCPProxy(localAddr, tdb.Addr())
		assert.NoError(t, proxy.Start(ctx))

		base := dao.New(&dbc.Config{
			Addr:     localAddr,
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 3,
			DrMode:   dr.Active,
		})
		defer base.Close()

		base.StartMonitor(ctx, 1*time.Second)
		base.StartCache(ctx, 10*time.Minute)

		authDao := NewAuthDao(base)
		accountLis := &model.Account{Username: "lis", Roles: []model.RoleName{"manager"}, Salt: "salksdj", Password: "tiger", Namespace: "_GLOBAL"}
		assert.NoError(t, authDao.SaveAccount(ctx, accountLis))

		{ // test request for account using master
			found, err := authDao.GetAccountByUsername(ctx, "lis")
			assert.NoError(t, err)
			assert.Equal(t, accountLis, found)
		}
		utils.CancelableSleep(ctx, 3*time.Second)

		fmt.Println("==================================================================")
		fmt.Println(" Interrupt connection to db ")
		fmt.Println("==================================================================")
		proxy.Close()

		// wait till base dao does not recognize availability error
		for i := 0; i < 20; i++ {
			if base.IsAvailable() != nil {
				// this is expected behavior
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
		assert.NotNil(t, base.IsAvailable())

		{ // retry request for account (should be done using cache)
			found, err := authDao.GetAccountByUsername(ctx, "lis")
			assert.NoError(t, err)
			assert.Equal(t, accountLis, found)
		}

		// check update request error type from dao
		// should be substituted on descriptive answer about read-only mode
		accountSergey := &model.Account{Username: "sergey", Roles: []model.RoleName{"manager"}, Salt: "eyteyr", Password: "topsecret", Namespace: "_GLOBAL"}
		assert.ErrorIs(t, authDao.SaveAccount(ctx, accountSergey), dao.MasterDatabaseUnavailableForUpdate)

		// emulate update in via other container that didn't lost connectivity to pg
		// update database data using other dao instance and check resync feature in testing dao instance
		base2 := dao.New(&dbc.Config{
			Addr:     tdb.Addr(),
			User:     tdb.Username(),
			Password: tdb.Password(),
			Database: tdb.DBName(),
			PoolSize: 3,
			DrMode:   dr.Active,
		})
		defer base2.Close()

		authDao2 := NewAuthDao(base2)
		accountEfimov := &model.Account{Username: "efimov", Roles: []model.RoleName{"manager"}, Salt: "itotp", Password: "pinkfloyd", Namespace: "_GLOBAL"}
		assert.NoError(t, authDao2.SaveAccount(ctx, accountEfimov))

		// enable tcp proxy
		assert.NoError(t, proxy.Start(ctx))
		fmt.Println("==================================================================")
		fmt.Println(" Connection to db is restored ")
		fmt.Println("==================================================================")
		utils.CancelableSleep(ctx, 10*time.Second)

		assert.NoError(t, base.IsAvailable())

		{ //  get data
			found, err := authDao.GetAccountByUsername(ctx, "efimov")
			assert.NoError(t, err)
			assert.Equal(t, accountEfimov, found)
		}

		proxy.Close()
	})
}

func TestAuthServiceImpl_IsAccessGranted(t *testing.T) {
	user := "test-user"
	password := utils.SecretString("my-super-secured-password")
	firstNamespace := "first"
	secondNamespace := "second"
	controllerNamespace := "controller"

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		dao := NewAuthDao(baseDao)
		bgDomainService := domain.NewBGDomainService(domain.NewBGDomainDao(baseDao))
		authService := NewAuthService(dao, nil, bgDomainService)

		_, err := authService.CreateUserAccount(ctx, &model.ClientAccountDto{
			Username:  user,
			Password:  password,
			Namespace: "first",
			Roles:     []model.RoleName{model.AgentRole},
		})
		assert.NoError(t, err)

		isAccessGrantedToFirstNamespace, err := authService.IsAccessGranted(ctx, user, password, firstNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToFirstNamespace)

		isAccessGrantedToSecondNamespace, err := authService.IsAccessGranted(ctx, user, password, secondNamespace, []model.RoleName{model.AgentRole})
		assert.ErrorIs(t, err, msg.AuthError)
		assert.Nil(t, isAccessGrantedToSecondNamespace)

		err = bgDomainService.Bind(ctx, firstNamespace, secondNamespace, controllerNamespace)
		assert.NoError(t, err)

		isAccessGrantedToFirstNamespace, err = authService.IsAccessGranted(ctx, user, password, firstNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToFirstNamespace)

		isAccessGrantedToSecondNamespace, err = authService.IsAccessGranted(ctx, user, password, secondNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToSecondNamespace)

		_, err = bgDomainService.Unbind(ctx, secondNamespace)
		assert.NoError(t, err)
		isAccessGrantedToFirstNamespace, err = authService.IsAccessGranted(ctx, user, password, secondNamespace, []model.RoleName{model.AgentRole})
		assert.ErrorIs(t, err, msg.AuthError)
		assert.Nil(t, isAccessGrantedToFirstNamespace)
	})
}

type mockVerifier struct {
	token     string
	username  string
	namespace string
}

func (mv mockVerifier) Verify(ctc context.Context, rawToken string) (*oidc.Claims, error) {
	if mv.token != rawToken {
		return nil, errors.Join(errors.New("invalid token"), msg.AuthError)
	}
	return &oidc.Claims{
		Kubernetes: oidc.K8sClaims{
			Namespace: mv.namespace,
			ServiceAccount: oidc.ServiceAccount{
				Name: mv.username,
			},
		},
	}, nil
}

func TestAuthServiceImpl_IsAccessGrantedWithToken(t *testing.T) {
	firstNamespace := "first"
	secondNamespace := "second"
	controllerNamespace := "controller"

	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		username := "test-user"
		token := "my-super-secured-password"

		verifier := mockVerifier{
			token:     token,
			username:  username,
			namespace: firstNamespace,
		}
		dao := NewAuthDao(baseDao)
		bgDomainService := domain.NewBGDomainService(domain.NewBGDomainDao(baseDao))
		authService := NewAuthService(dao, nil, bgDomainService)

		isAccessGrantedToFirstNamespace, err := authService.IsAccessGrantedWithToken(ctx, verifier, token, firstNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToFirstNamespace)

		isAccessGrantedToSecondNamespace, err := authService.IsAccessGrantedWithToken(ctx, verifier, token, secondNamespace, []model.RoleName{model.AgentRole})
		assert.ErrorIs(t, err, msg.AuthError)
		assert.Nil(t, isAccessGrantedToSecondNamespace)

		err = bgDomainService.Bind(ctx, firstNamespace, secondNamespace, controllerNamespace)
		assert.NoError(t, err)

		isAccessGrantedToFirstNamespace, err = authService.IsAccessGrantedWithToken(ctx, verifier, token, firstNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToFirstNamespace)

		isAccessGrantedToSecondNamespace, err = authService.IsAccessGrantedWithToken(ctx, verifier, token, secondNamespace, []model.RoleName{model.AgentRole})
		assert.NoError(t, err)
		assert.NotNil(t, isAccessGrantedToSecondNamespace)

		_, err = bgDomainService.Unbind(ctx, secondNamespace)
		assert.NoError(t, err)
		isAccessGrantedToFirstNamespace, err = authService.IsAccessGrantedWithToken(ctx, verifier, token, secondNamespace, []model.RoleName{model.AgentRole})
		assert.ErrorIs(t, err, msg.AuthError)
		assert.Nil(t, isAccessGrantedToFirstNamespace)
	})
}

func TestAuthServiceImpl_CheckSecurityForBoundNamespaces(t *testing.T) {
	dao.WithSharedDao(t, func(baseDao *dao.BaseDaoImpl) {
		ctx, cancelContext := context.WithCancel(context.Background())
		defer cancelContext()

		dao := NewAuthDao(baseDao)
		bgDomainService := domain.NewBGDomainService(domain.NewBGDomainDao(baseDao))

		authService := NewAuthService(dao, composite.NewPGRegistrationDao(baseDao), bgDomainService)

		err := authService.CheckSecurityForBoundNamespaces(ctx, "test-1-1", &model.Classifier{
			Name:      "tst-name",
			Namespace: "test-1-1",
		})
		assert.NoError(t, err)

		err = authService.CheckSecurityForBoundNamespaces(ctx, "test-1-1", &model.Classifier{
			Name:      "tst-name",
			Namespace: "test-1-2",
		})
		assert.ErrorIs(t, err, msg.AuthError)

		err = bgDomainService.Bind(ctx, "test-1-1", "test-1-2", "controller")
		assert.NoError(t, err)

		err = authService.CheckSecurityForBoundNamespaces(ctx, "test-1-1", &model.Classifier{
			Name:      "tst-name",
			Namespace: "test-1-1",
		})
		assert.NoError(t, err)

		err = authService.CheckSecurityForBoundNamespaces(ctx, "test-1-1", &model.Classifier{
			Name:      "tst-name",
			Namespace: "test-1-2",
		})
		assert.NoError(t, err)
	})
}
