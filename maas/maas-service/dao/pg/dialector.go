package pg

import (
	"context"
	"database/sql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/netcracker/qubership-maas/dr"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/callbacks"
	"regexp"
)

var timeZoneMatcher = regexp.MustCompile("(time_zone|TimeZone)=(.*?)($|&| )")

func MakeReadOnlyQuery() string {
	return "SET default_transaction_read_only TO on"
}

type DialectorWithCallbacks struct {
	postgres.Dialector
	drMode dr.Mode
}

func Open(dsn string, drMode dr.Mode) gorm.Dialector {
	return DialectorWithCallbacks{
		Dialector: postgres.Dialector{Config: &postgres.Config{DSN: dsn}},
		drMode:    drMode,
	}
}

func (d DialectorWithCallbacks) Initialize(db *gorm.DB) (err error) {
	// register callbacks
	if !d.WithoutReturning {
		callbacks.RegisterDefaultCallbacks(db, &callbacks.Config{
			CreateClauses: []string{"INSERT", "VALUES", "ON CONFLICT", "RETURNING"},
			UpdateClauses: []string{"UPDATE", "SET", "WHERE", "RETURNING"},
			DeleteClauses: []string{"DELETE", "FROM", "WHERE", "RETURNING"},
		})
	}

	if d.Conn != nil {
		db.ConnPool = d.Conn
	} else if d.DriverName != "" {
		db.ConnPool, err = sql.Open(d.DriverName, d.Config.DSN)
	} else {
		var config *pgx.ConnConfig

		config, err = pgx.ParseConfig(d.Config.DSN)
		if err != nil {
			return
		}

		result := timeZoneMatcher.FindStringSubmatch(d.Config.DSN)
		if len(result) > 2 {
			config.RuntimeParams["timezone"] = result[2]
		}
		if d.drMode.IsActive() {
			db.ConnPool = stdlib.OpenDB(*config)
		} else {
			makeReadOnly := func() stdlib.OptionOpenDB {
				return stdlib.OptionAfterConnect(func(ctx context.Context, conn *pgx.Conn) error {
					_, err := conn.Exec(ctx, MakeReadOnlyQuery())
					return err
				})
			}
			db.ConnPool = stdlib.OpenDB(*config, makeReadOnly())
		}
	}
	return
}
