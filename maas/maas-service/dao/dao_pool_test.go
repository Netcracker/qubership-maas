package dao

import (
	"database/sql"
	"fmt"
	"github.com/netcracker/qubership-maas/testharness"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConnectionPoolBehavior tests the connection pool behavior
// with different idle connection settings
func TestConnectionPoolBehavior(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		t.Run("DefaultSettings_BrokenPool", func(t *testing.T) {
			testConnectionPoolWithSettings(t, tdb, false)
		})

		t.Run("ProperSettings_WorkingPool", func(t *testing.T) {
			testConnectionPoolWithSettings(t, tdb, true)
		})
	})
}

func testConnectionPoolWithSettings(t *testing.T, tdb *testharness.TestDatabase, useProperSettings bool) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		tdb.Host(), tdb.Port(), tdb.Username(), tdb.Password(), tdb.DBName())

	sqlDb, err := sql.Open("postgres", dsn)
	require.NoError(t, err)
	defer sqlDb.Close()

	sqlDb.SetMaxOpenConns(3)
	sqlDb.SetConnMaxLifetime(30 * time.Minute)

	if useProperSettings {
		sqlDb.SetMaxIdleConns(3)
		sqlDb.SetConnMaxIdleTime(30 * time.Minute)
	} else {
		sqlDb.SetMaxIdleConns(0) // This will close connections immediately
		sqlDb.SetConnMaxIdleTime(0)
	}

	_, err = sqlDb.Exec("SELECT 1")
	require.NoError(t, err, "Database connection should be available")

	initialStats := sqlDb.Stats()
	t.Logf("Initial stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, MaxIdleClosed=%d, MaxIdleTimeClosed=%d",
		initialStats.OpenConnections, initialStats.InUse, initialStats.Idle,
		initialStats.WaitCount, initialStats.MaxIdleClosed, initialStats.MaxIdleTimeClosed)

	for i := 0; i < 5; i++ {
		_, err := sqlDb.Exec("SELECT 1")
		assert.NoError(t, err, "Database connection should be available")

		time.Sleep(100 * time.Millisecond)

		stats := sqlDb.Stats()
		t.Logf("Health check %d stats: Open=%d, InUse=%d, Idle=%d, WaitCount=%d, MaxIdleClosed=%d, MaxIdleTimeClosed=%d",
			i+1, stats.OpenConnections, stats.InUse, stats.Idle,
			stats.WaitCount, stats.MaxIdleClosed, stats.MaxIdleTimeClosed)

		if useProperSettings {
			// Connections should stay open and be reused
			assert.GreaterOrEqual(t, stats.OpenConnections, 1, "Should have at least 1 open connection")
			assert.Equal(t, int64(0), stats.WaitCount, "Should not have waited for connections")
		} else {
			// With broken settings, connections might be closed and reopened
			t.Logf("Broken pool behavior: Open=%d, MaxIdleClosed=%d", stats.OpenConnections, stats.MaxIdleClosed)
		}
	}
}
