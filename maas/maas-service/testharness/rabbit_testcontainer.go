package testharness

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

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
		Image:        "rabbitmq:3.13.6-management",
		ExposedPorts: []string{"15672/tcp", "5672/tcp"},
		WaitingFor:   wait.ForLog(".*startup complete.*").AsRegexp().WithOccurrence(1).WithStartupTimeout(5 * time.Minute),
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
		tdb.apiPort = int(p.Num())
	}

	{ // get port
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		p, err := rabbit.MappedPort(ctx, "5672")
		require.NoError(t, err)
		tdb.amqpPort = int(p.Num())
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

	// Try to terminate the container with retry logic
	var err error
	for i := 0; i < 3; i++ {
		err = db.instance.Terminate(ctx)
		if err == nil {
			return
		}
		// Container already gone — either AutoRemove fired or another cleanup path ran first
		if strings.Contains(err.Error(), "removal of container") && strings.Contains(err.Error(), "is already in progress") ||
			strings.Contains(err.Error(), "No such container") {
			t.Logf("Container already removed, continuing...")
			return
		}
		if i < 2 {
			time.Sleep(time.Second)
		}
	}
	require.NoError(t, err)
}
