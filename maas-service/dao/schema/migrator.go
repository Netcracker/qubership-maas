package schema

import (
	"context"
	core_tls "crypto/tls"
	"github.com/go-pg/migrations/v8"
	pgv10 "github.com/go-pg/pg/v10"
	"github.com/netcracker/qubership-maas/dao/db"
	"github.com/netcracker/qubership-maas/dao/pg"
	"time"
)

func Migrate(dbConfig *db.Config) {
	poolOptions := pgv10.Options{
		Addr:     dbConfig.Addr,
		User:     dbConfig.User,
		Password: dbConfig.Password,
		Database: dbConfig.Database,

		MaxRetries:      5,
		MinRetryBackoff: -1,

		DialTimeout:  30 * time.Second,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,

		PoolSize:           2,
		MaxConnAge:         10 * time.Second,
		PoolTimeout:        30 * time.Second,
		IdleTimeout:        10 * time.Second,
		IdleCheckFrequency: 100 * time.Millisecond,
	}
	if dbConfig.TlsEnabled {
		host, _ := dbConfig.HostPort()
		poolOptions.TLSConfig = &core_tls.Config{
			ServerName:         host,
			InsecureSkipVerify: dbConfig.TlsSkipVerify,
		}
	}
	if !dbConfig.DrMode.IsActive() {
		poolOptions.OnConnect = func(ctx context.Context, cn *pgv10.Conn) error {
			_, err := cn.Exec(pg.MakeReadOnlyQuery())
			return err
		}
	}
	dbPg := pgv10.Connect(&poolOptions)
	log.Info("Run db evolutions on: %s...", dbConfig.Addr)

	if dbConfig.DrMode.IsActive() {
		if _, _, err := migrations.Run(dbPg, "init"); err != nil {
			log.Panic("Error run init db: %v", err.Error())
		}
	} else {
		log.Info("Migrator init command skipped due dr-mode: %v", dbConfig.DrMode)
	}

	if oldVersion, newVersion, err := migrations.Run(dbPg, "up"); err != nil {
		log.Panic("Error run db evolution: %v, dr-mode: %v", err.Error(), dbConfig.DrMode)
	} else if oldVersion != newVersion {
		log.Info("Db version is evolved from %v to %v", oldVersion, newVersion)
	} else {
		log.Info("Db is in recent version: %v", oldVersion)
	}
	dbPg.Close()
}
