package db

import (
	"fmt"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/utils"
	"log"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Addr          string
	User          string
	Password      string `fmt:"obfuscate"`
	Database      string
	PoolSize      int
	ConnectionTtl time.Duration
	DrMode        dr.Mode
	TlsEnabled    bool
	TlsSkipVerify bool
	CipherKey     string `fmt:"obfuscate"`
}

func (cfg Config) Format(state fmt.State, verb int32) {
	utils.FormatterUtil(cfg, state, verb)
}

func (cfg Config) HostPort() (string, int) {
	parts := strings.Split(cfg.Addr, ":")
	switch len(parts) {
	case 1:
		return parts[0], 5432
	case 2:
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			log.Panicf("port must be a positive number in addr: `%s', error: %s", cfg.Addr, err.Error())
		}
		return parts[0], port
	default:
		panic("Incorrect addr format: `" + cfg.Addr + "', expected: host[:port]")
	}
}
