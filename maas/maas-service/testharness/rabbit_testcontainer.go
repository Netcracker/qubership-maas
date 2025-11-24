package testharness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type TestRabbit struct {
	user     string
	password string
	instance testcontainers.Container
	apiPort  int
	amqpPort int
	host     string
}

func WithNewTestRabbit(t *testing.T, testRunnable func(rabbit *TestRabbit)) {
	tdb := newTestRabbit(t)
	defer tdb.Close(t)
	testRunnable(tdb)
}

func newTestRabbit(t *testing.T) *TestRabbit {
	assertEnvironment()

	tdb := &TestRabbit{
		user:     "guest",
		password: "guest",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3-management",
		ExposedPorts: []string{"15672/tcp", "5672/tcp"},
		HostConfigModifier: func(config *container.HostConfig) {
			config.AutoRemove = true
		},
		WaitingFor: wait.ForLog(".*startup complete.*").AsRegexp().WithOccurrence(1).WithStartupTimeout(5 * time.Minute),
	}
	rabbit, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	_, _, err = rabbit.Exec(ctx, []string{"rabbitmq-plugins", "enable", "rabbitmq_shovel"})
	require.NoError(t, err)

	_, _, err = rabbit.Exec(ctx, []string{"rabbitmq-plugins", "enable", "rabbitmq_shovel_management"})
	require.NoError(t, err)

	tdb.instance = rabbit

	{ // get host
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		tdb.host, err = rabbit.Host(ctx)
		require.NoError(t, err)
	}

	{ // get port
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p, err := rabbit.MappedPort(ctx, "15672")
		require.NoError(t, err)
		tdb.apiPort = p.Int()
	}

	{ // get port
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p, err := rabbit.MappedPort(ctx, "5672")
		require.NoError(t, err)
		tdb.amqpPort = p.Int()
	}

	t.Logf("Rabbit test container endpoint: %+v\n", tdb)
	return tdb
}

func (db *TestRabbit) ApiUrl() string {
	return fmt.Sprintf("http://%v:%v/api", db.host, db.apiPort)
}
func (db *TestRabbit) AmqpUrl() string {
	return fmt.Sprintf("amqp://%v:%v", db.host, db.amqpPort)
}

func (db *TestRabbit) Close(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	err := db.instance.Terminate(ctx)
	if err != nil {
		errStr := err.Error()
		if strings.Contains(errStr, "removal of container") && strings.Contains(errStr, "is already in progress") {
			return
		}
		require.NoError(t, err)
	}
}
