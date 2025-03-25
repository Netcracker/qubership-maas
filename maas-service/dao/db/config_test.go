package db

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"maas/maas-service/dr"
	"testing"
	"time"
)

func Test_HostPortParse(t *testing.T) {
	cfg := Config{
		Addr:          "server:5432",
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		PoolSize:      10,
		DrMode:        dr.Active,
		TlsEnabled:    false,
		TlsSkipVerify: false,
	}

	h, p := cfg.HostPort()
	assert.Equal(t, "server", h)
	assert.Equal(t, 5432, p)
}

func Test_HostPortDefault(t *testing.T) {
	cfg := Config{
		Addr:          "server",
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		PoolSize:      10,
		DrMode:        dr.Active,
		TlsEnabled:    false,
		TlsSkipVerify: false,
	}

	h, p := cfg.HostPort()
	assert.Equal(t, "server", h)
	assert.Equal(t, 5432, p)
}
func Test_HostPortNAN(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	cfg := Config{
		Addr:          "server:wrong",
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		PoolSize:      10,
		DrMode:        dr.Active,
		TlsEnabled:    false,
		TlsSkipVerify: false,
	}

	cfg.HostPort()
}

func Test_Format(t *testing.T) {
	cfg := Config{
		Addr:          "server:5432",
		User:          "postgres",
		Password:      "postgres",
		Database:      "postgres",
		PoolSize:      10,
		ConnectionTtl: 5 * time.Second,
		DrMode:        0,
		TlsEnabled:    false,
		TlsSkipVerify: false,
		CipherKey:     "test-key",
	}

	assert.Equal(t,
		"{Addr:server:5432 User:postgres Password:*** Database:postgres PoolSize:10 ConnectionTtl:5s DrMode:active TlsEnabled:false TlsSkipVerify:false CipherKey:***}",
		fmt.Sprintf("%+v", cfg))
}
