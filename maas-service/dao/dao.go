package dao

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dlmiddlecote/sqlstats"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/dao/db"
	pg2 "github.com/netcracker/qubership-maas/dao/pg"
	"github.com/netcracker/qubership-maas/dao/replicator"
	"github.com/netcracker/qubership-maas/dao/schema"
	"github.com/netcracker/qubership-maas/dao/serializer"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/netcracker/qubership-maas/encryption"
	"github.com/netcracker/qubership-maas/utils"
	"github.com/prometheus/client_golang/prometheus"
	"gorm.io/gorm"
	gschema "gorm.io/gorm/schema"
	"hash/fnv"
	"strings"
	"time"
)

var log logging.Logger

func init() {
	log = logging.GetLogger("dao")
}

var ErrEntityAlreadyExists = errors.New("dao: entity already exists")
var MasterDatabaseUnavailableForUpdate = errors.New("master database is unavailable. Check PG availability. Service use data cache and supports all requests ONLY in READ-ONLY mode")

//go:generate mockgen -source=dao.go -destination=mock_dao/mock.go
type BaseDao interface {
	StartMonitor(ctx context.Context, masterMonitorCheckInterval time.Duration) error
	StartCache(ctx context.Context, fullCacheResyncInterval time.Duration) error

	WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error
	IsAvailable() error
	UsingDb(ctx context.Context, f func(conn *gorm.DB) error) error
	WithTx(ctx context.Context, f func(ctx context.Context, conn *gorm.DB) error) error
	DSN() string

	IsUniqIntegrityViolation(err error) bool
	IsForeignKeyIntegrityViolation(err error) bool
	Close()
}

type BaseDaoImpl struct {
	gorm          *gorm.DB
	masterMonitor *replicator.MasterMonitor
	replica       *replicator.BackupStorageReplicator
	cancel        context.CancelFunc
	dsn           string
	drMode        dr.Mode

	metrics metrics
}

type metrics struct {
	masterSucceededRequests  prometheus.Counter
	masterFailedRequests     prometheus.Counter
	replicaSucceededRequests prometheus.Counter
	replicaFailedRequests    prometheus.Counter
}

var ClassifierUniqIndexErr = errors.New("classifier unique violation")

var InstanceDesignatorUniqueIndexErr = errors.New("instance designator unique violation")
var InstanceDesignatorForeignKeyErr = errors.New("instance designator foreign key instance violation")

type InstanceError struct {
	Err     error
	Message string
}

func (e InstanceError) Error() string {
	return fmt.Sprintf("Instance error, error: '%s', message: (%s)", e.Err.Error(), e.Message)
}

func New(dbConfig *db.Config) *BaseDaoImpl {
	gschema.RegisterSerializer("aesb64", serializer.NewEncrypted(encryption.NewAes(dbConfig.CipherKey)))
	schema.Migrate(dbConfig)

	log.Info("Create db connection pool to `%v', user: `%v', poolSize: %d", dbConfig.Addr, dbConfig.User, dbConfig.PoolSize)
	host, port := dbConfig.HostPort()
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%d", host, dbConfig.User, dbConfig.Password, dbConfig.Database, port)
	if dbConfig.TlsEnabled {
		if dbConfig.TlsSkipVerify {
			dsn = dsn + " sslmode=prefer"
		} else {
			dsn = dsn + " sslmode=verify-full"
		}
	}

	sqlExecMetric := prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "maas",
		Subsystem: "db",
		Name:      "sql_executions",
		Help:      "SQL queries execution time",
		Buckets:   []float64{0.05, 0.2, 0.5, 1.0},
	})
	if err := prometheus.Register(sqlExecMetric); err != nil {
		log.Errorf("error register sql execution metric: %v", err)
	}

	dbGorm, err := gorm.Open(pg2.Open(dsn, dbConfig.DrMode), &gorm.Config{
		Logger: &GormLogger{log, sqlExecMetric},
	})

	// setup pooling
	sqlDb, err := dbGorm.DB()
	if err != nil {
		log.Panic("gorm.Open: Error : %s\n", err)
	}
	sqlDb.SetMaxOpenConns(dbConfig.PoolSize)
	sqlDb.SetConnMaxLifetime(dbConfig.ConnectionTtl)

	return &BaseDaoImpl{
		gorm:    dbGorm,
		dsn:     dsn,
		drMode:  dbConfig.DrMode,
		metrics: registerMetrics(sqlDb, dbConfig.DrMode),
	}
}

func (d *BaseDaoImpl) StartMonitor(ctx context.Context, masterMonitorCheckInterval time.Duration) error {
	d.masterMonitor = replicator.NewMasterMonitor(d.gorm, masterMonitorCheckInterval)
	d.masterMonitor.Start(ctx)
	return nil
}

func (d *BaseDaoImpl) StartCache(ctx context.Context, fullCacheResyncInterval time.Duration) error {
	if d.masterMonitor == nil {
		panic("master monitor not yet started")
	}
	log.InfoC(ctx, "Start backup cache replicator instance...")
	ctx, d.cancel = context.WithCancel(ctx)

	log.InfoC(ctx, "Start backup storage replicator...")
	d.replica = replicator.NewBackupStorageReplicator(d.dsn, d.gorm, d.masterMonitor)
	return d.replica.Start(ctx, fullCacheResyncInterval)
}

// see https://www.postgresql.org/docs/11/errcodes-appendix.html
func (d *BaseDaoImpl) IsUniqIntegrityViolation(err error) bool {
	if strings.Contains(err.Error(), "23505") { // ugly, but github.com/go-pg/pg/internal.PGError is internal
		return true
	}
	return false
}

func (d *BaseDaoImpl) IsForeignKeyIntegrityViolation(err error) bool {
	if strings.Contains(err.Error(), "23503") {
		return true
	}
	return false
}

func (d *BaseDaoImpl) DSN() string {
	return d.dsn
}

type Lock struct {
	Id string
}

func (d *BaseDaoImpl) WithLock(ctx context.Context, lockId string, f func(ctx context.Context) error) error {
	h := fnv.New32()
	h.Write([]byte(lockId))

	return d.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
		if err := conn.Exec("select pg_advisory_xact_lock(?)", h.Sum32()).Error; err != nil {
			return utils.LogError(log, ctx, "error acquire lock: %w", err)
		}

		return f(ctx)
	})

	return d.WithTx(ctx, func(ctx context.Context, conn *gorm.DB) error {
		if err := conn.Exec("select pg_advisory_xact_lock(?)", h.Sum32()).Error; err != nil {
			return utils.LogError(log, ctx, "error acquire lock: %w", err)
		}

		return f(ctx)
	})
}

func (d *BaseDaoImpl) Close() {
	log.Info("Shutdown db connection pool")

	if d.cancel != nil {
		d.cancel() // cancel backup and monitoring routines
	}

	cnn, err := d.gorm.DB()
	if err == nil {
		_ = cnn.Close()
	} else {
		log.Info("Error get db from gorm: %s", err)
	}
}

func (d *BaseDaoImpl) db() *gorm.DB {
	return d.gorm
}

func (d *BaseDaoImpl) IsAvailable() error {
	return d.masterMonitor.Status()
}

func (d *BaseDaoImpl) UsingDb(ctx context.Context, f func(conn *gorm.DB) error) error {
	executedOnMaster := false
	var err error
	// try request on master db if it available or cache not started
	if d.masterMonitor == nil || d.masterMonitor.IsAlive() {
		tx := TransactionContextOf(ctx)
		cnn := d.gorm

		// if we in context of global transaction, then use connection from them
		if tx != nil && tx.TxCnn() != nil {
			cnn = tx.TxCnn()
		}

		executedOnMaster = true
		err = withMetrics(
			d.metrics.masterSucceededRequests,
			d.metrics.masterFailedRequests,
			func() error { return f(cnn.WithContext(ctx)) },
		)
	}

	// query wasn't executed at all or executed on master but has and an error caused by master availability
	if (!executedOnMaster || (err != nil && IsMasterAvailabilityError(err))) &&
		// and we have an backup data replica run
		d.replica != nil {

		log.DebugC(ctx, "detected connectivity problems to master db, retry on in-memory cache")
		// retry request on slave if it exists
		err = withMetrics(
			d.metrics.replicaSucceededRequests,
			d.metrics.replicaFailedRequests,
			func() error { return f(d.replica.DB().WithContext(ctx)) },
		)

		if d.replica.IsReadOnlyError(err) {
			// user attempts to perform create/update/delete operations on read-only cache
			return MasterDatabaseUnavailableForUpdate
		}
	}

	return d.translateDRModeError(err)
}

func (d *BaseDaoImpl) WithTx(ctx context.Context, f func(ctx context.Context, conn *gorm.DB) error) error {
	exec := func(ctx context.Context) error {
		return d.UsingDb(ctx, func(cnn *gorm.DB) error {
			return f(ctx, TransactionContextOf(ctx).Begin(cnn))
		})
	}

	tx := TransactionContextOf(ctx)
	if tx == nil {
		// create local tx
		// commit exactly at the end of operation
		log.DebugC(ctx, "Execute using local transaction")
		return WithTransactionContext(ctx, exec)
	} else {
		// using transaction already presented in context
		// do not perform commit here
		return exec(ctx)
	}
}

// IsMasterAvailabilityError
//
//	 it's absolutely unclear how to distinguish connectivity errors from syntax or data errors
//		so I hope we enhance this clause using our future experience
func IsMasterAvailabilityError(err error) bool {
	return err != nil && err.Error() == "unexpected EOF"

	// here errors that PG defines as leading to connectivity problems
	//return strings.HasPrefix(code, "08") ||
	//	strings.HasPrefix(code, "53") ||
	//	strings.HasPrefix(code, "57") ||
	//	strings.HasPrefix(code, "58") ||
	//	strings.HasPrefix(code, "F0") ||
	//	strings.HasPrefix(code, "XX")
}

func registerMetrics(sqlDb *sql.DB, drMode dr.Mode) metrics {
	// show dr mode
	drModeCollector := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "maas",
		Subsystem: "db",
		Name:      "dr_mode",
		Help:      "current DR execution mode: Active=0, Standby=1, Disabled=2",
	})
	if err := prometheus.Register(drModeCollector); err != nil {
		log.Errorf("Error registering metric maas_db_dr_mode: %v", err)
	}
	drModeCollector.Set(float64(drMode))

	// show pool statistics
	poolCollector := sqlstats.NewStatsCollector("maas", sqlDb)
	if err := prometheus.Register(poolCollector); err != nil {
		log.Errorf("Error registering pool metrics: %v", err)
	}

	// UsingDb counters
	register := func(labels map[string]string) prometheus.Counter {
		collector := prometheus.NewCounter(prometheus.CounterOpts{
			Namespace:   "maas",
			Subsystem:   "db",
			Name:        "request_count",
			Help:        "number of processed sql requests",
			ConstLabels: labels,
		})
		if err := prometheus.Register(collector); err != nil {
			log.Errorf("Error registering metric maas_db_request_count with labels: %+v: %w", labels, err)
		}
		return collector
	}

	return metrics{
		masterSucceededRequests:  register(map[string]string{"type": "master", "result": "success"}),
		masterFailedRequests:     register(map[string]string{"type": "master", "result": "error"}),
		replicaSucceededRequests: register(map[string]string{"type": "replica", "result": "success"}),
		replicaFailedRequests:    register(map[string]string{"type": "replica", "result": "error"}),
	}
}

func withMetrics(onSuccessCounter, onErrorCounter prometheus.Counter, execution func() error) error {
	err := execution()
	if err == nil {
		onSuccessCounter.Add(1)
	} else {
		onErrorCounter.Add(1)
	}
	return err
}
