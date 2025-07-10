package replicator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	go_sqlite "github.com/glebarez/go-sqlite"
	"github.com/glebarez/sqlite"
	"github.com/jackc/pgx/v5"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/utils"
	"gorm.io/gorm"
	"reflect"
	"regexp"
	"strconv"
	"sync"
	"time"
)

var log = logging.GetLogger("backup-replicator")

type BackupStorageReplicator struct {
	logger    logging.Logger
	masterDsn string
	master    *gorm.DB
	slave     *gorm.DB
	slaveRO   *gorm.DB
	dbName    string
	metadata  map[string][]columnMetadata
	cancel    context.CancelFunc
	replWG    *sync.WaitGroup
	monitor   *MasterMonitor
}

type columnMetadata struct {
	name     string
	datatype string
}

type syncEventPayload struct {
	Table  string                 `json:"table"`
	Action string                 `json:"action"`
	Old    map[string]interface{} `json:"old"`
	New    map[string]interface{} `json:"new"`
}

func NewBackupStorageReplicator(dsn string, master *gorm.DB, monitor *MasterMonitor) *BackupStorageReplicator {
	match := regexp.MustCompile(`dbname=(\w+)`).FindStringSubmatch(dsn)
	if len(match) != 2 {
		log.Panic("can't extract database name from dsn: %v", dsn)
	}
	dbName := match[1]

	slave, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	if err != nil {
		panic(fmt.Errorf("error open sqlite database: %w", err))
	}

	slaveRO, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"))
	if err != nil {
		panic(fmt.Errorf("error open sqlite database: %w", err))
	}
	if err := slaveRO.Exec("PRAGMA query_only=true").Error; err != nil {
		panic(fmt.Errorf("error make sqlite database read-only: %w", err))
	}

	return &BackupStorageReplicator{
		logging.GetLogger("backup-storage"),
		dsn,
		master,
		slave,
		slaveRO,
		dbName,
		nil,
		nil,
		nil,
		monitor,
	}
}

func (bsr *BackupStorageReplicator) DB() *gorm.DB {
	return bsr.slaveRO
}

func (bsr *BackupStorageReplicator) Start(ctx context.Context, fullResyncInterval time.Duration) error {
	bsr.logger.InfoC(ctx, "Start full sync")

	if bsr.metadata == nil {
		var err error
		if bsr.metadata, err = bsr.getSchemaMetadata(ctx); err != nil {
			return fmt.Errorf("error getting schema metadata: %w", err)
		}

		bsr.logger.DebugC(ctx, "Schema: %+v", bsr.metadata)

		if err = bsr.recreateSlaveTables(ctx, bsr.metadata); err != nil {
			return fmt.Errorf("error create schema on slave: %w", err)
		}

		if err = bsr.attachTriggers(ctx, bsr.metadata); err != nil {
			return fmt.Errorf("error attaching data change triggers to tables: %w", err)
		}
	}

	bsr.logger.InfoC(ctx, "Full resync data to slave...")

	// stop previous listener
	if bsr.cancel != nil {
		bsr.cancel()
	}
	bsr.cancel = nil

	if err := bsr.cacheFullUpdate(ctx); err != nil {
		return fmt.Errorf("error initial cache creation: %w", err)
	}

	listenerCtx, cancel := context.WithCancel(ctx)

	if wg, err := bsr.startAsyncTasks(listenerCtx, bsr.masterDsn, fullResyncInterval); err == nil {
		bsr.cancel = func() {
			bsr.logger.DebugC(ctx, "Cancel previous master listener")
			cancel()
			wg.Wait()
			bsr.logger.InfoC(ctx, "Database event listener stopped")
		}
	} else {
		cancel()
		return fmt.Errorf("error start master data listener: %w", err)
	}

	bsr.logger.InfoC(ctx, "Cache sync successfully finished")
	return nil
}

func (bsr *BackupStorageReplicator) attachTriggers(ctx context.Context, metadata map[string][]columnMetadata) error {
	log.DebugC(ctx, "Create table update event triggers...")

	for tableName, _ := range metadata {
		log.DebugC(ctx, "Add table event trigger for: `%s'", tableName)
		deleteTriggerSql := fmt.Sprintf(`drop trigger if exists %[1]s_sync on %[1]s`, tableName)
		if err := bsr.master.Exec(deleteTriggerSql).Error; err != nil {
			return fmt.Errorf("error delete trigger on `%s': %w", tableName, err)
		}

		createTriggerSql := fmt.Sprintf(`create trigger %[1]s_sync after insert or delete or update on
				%[1]s for each row execute function maas_cache_sync_notify()`, tableName)
		if err := bsr.master.Exec(createTriggerSql).Error; err != nil {
			return fmt.Errorf("error create update capture trigger on `%s': %w", tableName, err)
		}
	}
	return nil
}

func (bsr *BackupStorageReplicator) getSchemaMetadata(ctx context.Context) (map[string][]columnMetadata, error) {
	bsr.logger.InfoC(ctx, "Collect schema metadata...")
	metadataRows, err := bsr.master.Raw(`SELECT c.table_name, c.column_name, c.data_type
		  FROM information_schema.columns as c inner join information_schema.tables as t on c.table_name = t.table_name
		 where 
			c.table_catalog = @dbName
			and c.table_schema = 'public'
			and c.table_name not in ('gopg_migrations', 'locks', '_dbaas_metadata', 'cache_sync_payload', 'entity_request_stat')
		 	and t.table_type = 'BASE TABLE'
		 order by c.table_name, c.ordinal_position`, map[string]interface{}{"dbName": bsr.dbName}).Rows()
	if err != nil {
		return nil, fmt.Errorf("error request table list: %w", err)
	}
	defer metadataRows.Close()

	metadata := make(map[string][]columnMetadata)
	for metadataRows.Next() {
		var tableName string
		var columnName string
		var columnType string
		metadataRows.Scan(&tableName, &columnName, &columnType)

		if columns, ok := metadata[tableName]; ok {
			metadata[tableName] = append(columns, columnMetadata{columnName, columnType})
		} else {
			metadata[tableName] = []columnMetadata{{columnName, columnType}}
		}
	}

	return metadata, nil
}

func (bsr *BackupStorageReplicator) recreateSlaveTables(ctx context.Context, metadata map[string][]columnMetadata) error {
	for tableName, columns := range metadata {
		sql := "create table " + tableName + " ("
		for i := range columns {
			if i > 0 {
				sql += ", "
			}
			sql += columns[i].name + " " + columns[i].datatype
		}
		sql += ")"

		bsr.logger.InfoC(ctx, "Create cache table: %v", tableName)
		if err := bsr.slave.Exec(sql).Error; err != nil {
			return fmt.Errorf("error create table: %v, error: %w", sql, err)
		}
	}

	return nil
}

func (bsr *BackupStorageReplicator) cacheFullUpdate(ctx context.Context) error {
	log.InfoC(ctx, "start full cache resync...")
	tx, err := bsr.lockMasterTables(ctx)
	if err != nil {
		return fmt.Errorf("error locking master tables: %w", err)
	}
	defer func() {
		bsr.logger.InfoC(ctx, "Release locks on master tables")
		tx.Rollback()
	}()
	masterDb, err := tx.DB()
	if err != nil {
		return fmt.Errorf("error getting connection to master db: %w", err)
	}

	slaveDb, err := bsr.slave.DB()
	if err != nil {
		return fmt.Errorf("error getting connection to slave db: %w", err)
	}

	slaveTx, err := slaveDb.Begin()
	if err != nil {
		return fmt.Errorf("error start tx on slave db: %w", err)
	}
	defer slaveTx.Rollback()

	for tableName, columns := range bsr.metadata {
		bsr.logger.InfoC(ctx, "Cache data for: %v", tableName)

		rows, err := masterDb.QueryContext(ctx, formatSelectSql(tableName, columns))
		if err != nil {
			if rows != nil {
				rows.Close()
			}

			return err
		}

		if _, err := slaveDb.ExecContext(ctx, "delete from "+tableName); err != nil {
			return fmt.Errorf("error truncate table: %s, error: %w", tableName, err)
		}

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		for rows.Next() {
			rows.Scan(valuePtrs...)

			insertSql := formatInsertSql(tableName, columns)
			if inserted, err := slaveDb.ExecContext(ctx, insertSql, values...); err != nil {
				rows.Close()
				return fmt.Errorf("error execute sql: %s, error: %w", insertSql, err)
			} else if count, _ := inserted.RowsAffected(); count != 1 {
				return fmt.Errorf("no rows inserted")
			}
		}
		rows.Close()
	}
	slaveTx.Commit()
	log.InfoC(ctx, "full cache resync successfully finished")

	return nil
}

func (bsr *BackupStorageReplicator) startAsyncTasks(ctx context.Context, dsn string, fullResyncInterval time.Duration) (*sync.WaitGroup, error) {
	tasksRunning := &sync.WaitGroup{}

	// full resync task
	tasksRunning.Add(1)
	go func() {
		log.InfoC(ctx, "Start full cache resync scheduler with interval: %s", fullResyncInterval)
		select {
		case <-time.After(fullResyncInterval):
			if bsr.monitor.IsAlive() {
				if err := bsr.cacheFullUpdate(ctx); err != nil {
					log.ErrorC(ctx, "Error cache full resync: %s", err)
				}
			}
		case <-ctx.Done():
			log.InfoC(ctx, "Full resync scheduler is stopped")
			tasksRunning.Done()
			return
		}
	}()

	// db notify listener
	syncChannel := make(chan int, 256)
	tasksRunning.Add(1)
	listenerStarted := make(chan string)
	go func() {
		defer func() {
			close(syncChannel)
			tasksRunning.Done()
			bsr.logger.InfoC(ctx, "Sync events collector is stopped")
		}()

		for {
			if !bsr.monitor.IsAlive() {
				if !utils.CancelableSleep(ctx, 5*time.Second) {
					return
				}
				continue
			}

			bsr.logger.InfoC(ctx, "Start sync events collector")
			if err := bsr.runEventListener(ctx, dsn, syncChannel, listenerStarted); err != nil {
				if IsContextCancelled(ctx) {
					return
				}
				bsr.logger.ErrorC(ctx, "sync error: %s", err)
				if !utils.CancelableSleep(ctx, 5*time.Second) {
					return
				}
			}
		}
	}()

	// db event replicator
	tasksRunning.Add(1)
	go func() {
		defer tasksRunning.Done()

		bsr.logger.InfoC(ctx, "Start sync events replicator")
		for {
			select {
			case <-ctx.Done():
				bsr.logger.InfoC(ctx, "Sync events replicator is stopped")
				return
			case updateId := <-syncChannel:
				if err := bsr.handleSyncEvent(ctx, updateId); err != nil {
					bsr.logger.ErrorC(ctx, "ignore replicate error: %s, update id: %s", err, updateId)
				}
			}
		}
	}()

	<-listenerStarted
	return tasksRunning, nil
}

func (bsr *BackupStorageReplicator) runEventListener(ctx context.Context, dsn string, ch chan<- int, listenerStarted chan<- string) error {
	cnn, err := pgx.Connect(ctx, dsn)
	if err != nil {
		return err
	}
	defer cnn.Close(ctx)

	bsr.logger.InfoC(ctx, "Call listen sync")
	_, err = cnn.Exec(ctx, "listen sync")
	listenerStarted <- "started"
	if err != nil {
		return err
	}

	for {
		bsr.logger.DebugC(ctx, "Wait for data update event")
		notification, err := cnn.WaitForNotification(ctx)
		if err != nil {
			if IsContextCancelled(ctx) {
				return ctx.Err() // context cancel called
			}
			return fmt.Errorf("error wait data change notification: %w", err)
		}
		bsr.logger.DebugC(ctx, "Receive data update event: %s", notification.Payload)
		updateId, err := strconv.Atoi(notification.Payload)
		if err != nil {
			return fmt.Errorf("invalid notification format: `%s', error: %w", notification.Payload, err)
		}
		ch <- updateId
	}
}

func (bsr *BackupStorageReplicator) handleSyncEvent(ctx context.Context, syncUpdateNumber int) error {
	bsr.logger.DebugC(ctx, "Event from db #%d", syncUpdateNumber)

	var rawPayload string
	res := bsr.master.Table("cache_sync_payload").
		Select("data").
		Where("id = ?", syncUpdateNumber).
		Find(&rawPayload)
	if res.Error != nil {
		return fmt.Errorf("error getting event payload from cache_sync_payload #%d: %w", syncUpdateNumber, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("no data found by sync number #%d", syncUpdateNumber)
	}

	bsr.logger.DebugC(ctx, "Sync cache by: %+v", rawPayload)
	var event syncEventPayload
	err := json.Unmarshal([]byte(rawPayload), &event)
	if err != nil {
		return fmt.Errorf("error unmarshal event `%s': %w", rawPayload, err)
	}

	var columns []columnMetadata
	var ok bool
	if columns, ok = bsr.metadata[event.Table]; !ok {
		return fmt.Errorf("event for unknown table: %s", event.Table)
	}

	flattenValue := func(raw interface{}) interface{} {
		v := reflect.ValueOf(raw)
		switch v.Kind() {
		case reflect.Slice, reflect.Map:
			data, err := json.Marshal(raw)
			if err != nil {
				panic(err)
			}
			return string(data)
		default:
			return raw
		}
	}

	var values []interface{}
	var sql string

	switch event.Action {
	case "INSERT":
		for _, column := range columns {
			values = append(values, flattenValue(event.New[column.name]))
		}
		sql = formatInsertSql(event.Table, columns)
	case "UPDATE":
		for _, column := range columns {
			values = append(values, flattenValue(event.New[column.name]))
		}
		for _, column := range columns {
			values = append(values, flattenValue(event.Old[column.name]))
		}
		sql = formatUpdateSql(event.Table, columns)
	case "DELETE":
		for _, column := range columns {
			values = append(values, flattenValue(event.Old[column.name]))
		}
		sql = formatDeleteSql(event.Table, columns)
	default:
		panic("Unexpected event type: " + event.Action)
	}

	return bsr.slave.Exec(sql, values...).Error
}

func (bsr *BackupStorageReplicator) lockMasterTables(ctx context.Context) (*gorm.DB, error) {
	bsr.logger.InfoC(ctx, "Lock master tables for sync start...")
	lockSql := ""
	for tableName := range bsr.metadata {
		if lockSql != "" {
			lockSql += ", "
		}
		lockSql += tableName
	}
	lockSql = "lock table " + lockSql + " in exclusive mode"
	tx := bsr.master.Begin(&sql.TxOptions{Isolation: sql.LevelReadCommitted, ReadOnly: true})
	// FIXME
	//if err := tx.Exec(lockSql).Error; err != nil {
	//	return nil, fmt.Errorf("error acquire exclusive locks on tables: %w", err)
	//}

	return tx, nil
}

func (bsr *BackupStorageReplicator) IsReadOnlyError(err error) bool {
	var sqliteError *go_sqlite.Error
	return errors.As(err, &sqliteError) && sqliteError.Code() == 8 /*SQLITE_READONLY*/
}

func formatInsertSql(tableName string, columns []columnMetadata) string {
	insertSqlStart := "insert into " + tableName + "("
	insertSqlValues := ") values ("
	for i := range columns {
		if i > 0 {
			insertSqlStart += ", "
			insertSqlValues += ", "
		}
		insertSqlStart += columns[i].name
		insertSqlValues += "$" + strconv.Itoa(i+1)
	}
	return insertSqlStart + insertSqlValues + ")"
}

func formatSelectSql(tableName string, columns []columnMetadata) string {
	selectSql := "select "
	for i := range columns {
		if i > 0 {
			selectSql += ", "
		}
		selectSql += "\"" + columns[i].name + "\""
	}

	selectSql += " from " + tableName
	return selectSql
}

func formatUpdateSql(tableName string, columns []columnMetadata) string {
	updateSql := "update " + tableName + " set "
	for i := range columns {
		if i > 0 {
			updateSql += ", "
		}
		updateSql += columns[i].name + "=$" + strconv.Itoa(i+1)
	}

	updateSql += " where "
	for i := range columns {
		if i > 0 {
			updateSql += " and "
		}
		updateSql += columns[i].name + "=$" + strconv.Itoa(i+1+len(columns))
	}
	return updateSql
}

func formatDeleteSql(tableName string, columns []columnMetadata) string {
	deleteSql := "delete from " + tableName + " where "
	for i := range columns {
		if i > 0 {
			deleteSql += " and "
		}
		deleteSql += columns[i].name + "=$" + strconv.Itoa(i+1)
	}

	return deleteSql
}

func IsContextCancelled(ctx context.Context) bool {
	return errors.Is(ctx.Err(), context.Canceled)
}
