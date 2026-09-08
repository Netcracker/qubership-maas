package dao

import (
	"context"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/netcracker/qubership-maas/dao/replicator"
	"github.com/netcracker/qubership-maas/dr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// masterIsGone is the error IsMasterAvailabilityError recognises.
var masterIsGone = errors.New("unexpected EOF")

// newCacheBackedDao builds a dao with a cache, so the fallback can be driven
// from the query closure.
func newCacheBackedDao(t *testing.T) *BaseDaoImpl {
	master, err := gorm.Open(sqlite.Open("file::memory:"))
	require.NoError(t, err)
	counter := func() prometheus.Counter {
		return prometheus.NewCounter(prometheus.CounterOpts{Name: "test"})
	}
	return &BaseDaoImpl{
		gorm:    master,
		replica: replicator.NewBackupStorageReplicator("host=localhost dbname=maas", master, nil),
		drMode:  dr.Active,
		metrics: metrics{
			masterSucceededRequests:  counter(),
			masterFailedRequests:     counter(),
			replicaSucceededRequests: counter(),
			replicaFailedRequests:    counter(),
		},
	}
}

// A query the SQLite cache cannot run failed on availability, not on input.
// Reported as anything else, a leader change comes back as a permanent 400.
func TestUsingDb_CacheFailureIsReportedAsUnavailable(t *testing.T) {
	baseDao := newCacheBackedDao(t)

	calls := 0
	err := baseDao.UsingDb(context.Background(), func(conn *gorm.DB) error {
		calls++
		if calls == 1 {
			return masterIsGone
		}
		return errors.New("SQL logic error: no such function: ANY")
	})

	require.ErrorIs(t, err, MasterDatabaseUnavailable)
	assert.Equal(t, 2, calls, "the query must be replayed on the cache")
	assert.Contains(t, err.Error(), "no such function: ANY", "the cache failure must stay in the message")
}

// A row the cache does not hold is an answer, not a failure of the cache.
func TestUsingDb_MissingRowOnCacheIsAnAnswer(t *testing.T) {
	baseDao := newCacheBackedDao(t)

	calls := 0
	err := baseDao.UsingDb(context.Background(), func(conn *gorm.DB) error {
		calls++
		if calls == 1 {
			return masterIsGone
		}
		return gorm.ErrRecordNotFound
	})

	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	assert.NotErrorIs(t, err, MasterDatabaseUnavailable)
}

// A query the master answers never reaches the cache.
func TestUsingDb_MasterAnswerIsNotReplayed(t *testing.T) {
	baseDao := newCacheBackedDao(t)

	calls := 0
	require.NoError(t, baseDao.UsingDb(context.Background(), func(conn *gorm.DB) error {
		calls++
		return nil
	}))
	assert.Equal(t, 1, calls)
}
