package testharness

import (
	"context"
	"fmt"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/stretchr/testify/assert"
	pgcontainer "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
)

type TestDatabase struct {
	instance testcontainers.Container
	user     string
	password string
	dbname   string
	port     int
	host     string

	shared bool
	closed bool
}

var (
	sharedOnce = sync.Once{}
	sharedDB   *TestDatabase
)

func assertEnvironment() {
	testDockerUrl, found := os.LookupEnv("TEST_DOCKER_URL")
	if found {
		if err := os.Setenv("DOCKER_HOST", testDockerUrl); err != nil {
			log.Panic("Failed to set env DOCKER_HOST=%s\n %v", testDockerUrl, err)
		}
	}
}

func WithSharedTestDatabase(t *testing.T, testRunnable func(*TestDatabase)) {
	assertEnvironment()

	sharedOnce.Do(func() {
		fmt.Println(">>> Create shared database container")
		sharedDB = newTestDatabase(t)
		sharedDB.shared = true
	})

	sandbox := createSandboxDatabase(sharedDB)
	testRunnable(sandbox)
}

func WithNewTestDatabase(t *testing.T, testRunnable func(*TestDatabase)) {
	tdb := newTestDatabase(t)
	defer tdb.Close(t)
	testRunnable(tdb)
}

func newTestDatabase(t *testing.T) *TestDatabase {
	assertEnvironment()

	tdb := &TestDatabase{
		user:     "postgres",
		password: "postgres",
		dbname:   "postgres",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	pg, err := pgcontainer.Run(ctx,
		"docker.io/postgres:15-alpine",
		pgcontainer.WithDatabase(tdb.dbname),
		pgcontainer.WithUsername(tdb.user),
		pgcontainer.WithPassword(tdb.password),
		testcontainers.WithWaitStrategy(
			wait.ForLog(".*database system is ready to accept connections.*").
				AsRegexp().
				WithOccurrence(2).
				WithStartupTimeout(1*time.Minute),
			wait.ForExposedPort(),
		),
	)
	require.NoError(t, err)

	tdb.instance = pg

	{ // get host
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		tdb.host, err = pg.Host(ctx)
		require.NoError(t, err)
	}

	{ // get port
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p, err := pg.MappedPort(ctx, "5432")
		require.NoError(t, err)
		tdb.port = p.Int()
	}

	t.Logf("PostgresSQL test container endpoint: %+v\n", tdb)
	return tdb
}

func createSandboxDatabase(containerdb *TestDatabase) *TestDatabase {
	main := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d",
		containerdb.host, containerdb.user, containerdb.password, containerdb.dbname, containerdb.port)
	db, err := gorm.Open(postgres.Open(main))
	if err != nil {
		panic(err)
	}

	suffix := strings.ReplaceAll(utils.UUID(), "-", "")
	sandbox := &TestDatabase{
		instance: containerdb.instance,
		user:     "u" + suffix,
		password: utils.UUID(),
		dbname:   "d" + suffix,
		port:     containerdb.port,
		host:     containerdb.host,
		shared:   containerdb.shared,
	}
	createUserSql := fmt.Sprintf("create user %s with password '%s'", sandbox.user, sandbox.password)
	if err := db.Exec(createUserSql).Error; err != nil {
		panic(err)
	}
	createDatabaseSql := fmt.Sprintf("create database %s", sandbox.dbname)
	if err := db.Exec(createDatabaseSql).Error; err != nil {
		panic(err)
	}
	makeOwnerSql := fmt.Sprintf(`alter database %s owner to %s`, sandbox.dbname, sandbox.user)
	if err := db.Exec(makeOwnerSql).Error; err != nil {
		panic(err)
	}
	return sandbox
}

func (db *TestDatabase) DSN() string {
	return fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d", db.host, db.user, db.password, db.dbname, db.port)
}

func (db *TestDatabase) Host() string {
	return db.host
}
func (db *TestDatabase) Port() int {
	return db.port
}
func (db *TestDatabase) Addr() string {
	return fmt.Sprintf("%s:%d", db.host, db.port)
}
func (db *TestDatabase) Username() string {
	return db.user
}
func (db *TestDatabase) Password() string {
	return db.password
}
func (db *TestDatabase) DBName() string {
	return db.dbname
}
func (db *TestDatabase) Close(t *testing.T) {
	if db.shared {
		// do not close container and rely on container reaper for cleanup
		return
	}

	if db.closed {
		// already closed
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := db.instance.Terminate(ctx)
	db.closed = true
	require.NoError(t, err)
}

func (db *TestDatabase) Gorm(t *testing.T) *gorm.DB {
	open, err := gorm.Open(postgres.Open(db.DSN()))
	if err != nil {
		assert.FailNowf(t, "error create gorm instance", "error: %v", err)
	}
	return open
}
