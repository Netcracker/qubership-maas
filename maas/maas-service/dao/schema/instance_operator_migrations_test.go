package schema

import (
	"testing"

	"github.com/go-pg/migrations/v8"
	pgv10 "github.com/go-pg/pg/v10"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"github.com/netcracker/qubership-maas/testharness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedRabbitUrlsError(t *testing.T) {
	err := sharedRabbitUrlsError([]sharedRabbitUrl{
		{Column: "api_url", Url: "http://rabbit:15672/api", Ids: "rabbit-infra, rabbit-old"},
		{Column: "amqp_url", Url: "amqp://rabbit:5672", Ids: "rabbit-infra, rabbit-old"},
	})
	assert.Equal(t, "rabbit_instances: URLs shared by several instances, upgrade stopped:\n"+
		"  api_url  http://rabbit:15672/api used by instances [rabbit-infra, rabbit-old]\n"+
		"  amqp_url amqp://rabbit:5672 used by instances [rabbit-infra, rabbit-old]\n"+
		"Leave one instance per URL, then restart maas-service.", err.Error())
}

func TestInstanceOperatorMigrations(t *testing.T) {
	testharness.WithNewTestDatabase(t, func(tdb *testharness.TestDatabase) {
		migrationCipherKey = configloader.GetKoanf().MustString("db.cipher.key")
		dbPg := connect(tdb)
		defer dbPg.Close()

		_, _, err := migrations.Run(dbPg, "init")
		require.NoError(t, err)
		_, _, err = migrations.Run(dbPg, "up", "30")
		require.NoError(t, err)

		// rows registered before the upgrade
		_, err = dbPg.Exec(`INSERT INTO kafka_instances (id, addresses, is_default) VALUES ('kafka-old', '{"PLAINTEXT": ["kafka:9092"]}', true)`)
		require.NoError(t, err)
		_, err = dbPg.Exec(`INSERT INTO rabbit_instances (id, api_url, amqp_url, "user", password, is_default) VALUES
			('rabbit-old', 'http://rabbit:15672/api', 'amqp://rabbit:5672', 'u', 'p', true),
			('rabbit-infra', 'http://rabbit:15672/api', 'amqp://rabbit:5672', 'u', 'p', false),
			('rabbit-other', 'http://other:15672/api', 'amqp://other:5672', 'u', 'p', false)`)
		require.NoError(t, err)

		// go-pg leaves the failed migration transaction open, holding the lock on the migrations table, until the
		// connection closes; maas-service panics and exits there, so run it on a pool that is closed afterwards
		failedPg := connect(tdb)
		_, _, err = migrations.Run(failedPg, "up")
		require.NoError(t, failedPg.Close())
		assert.ErrorContains(t, err, "rabbit_instances: URLs shared by several instances, upgrade stopped:\n"+
			"  api_url  http://rabbit:15672/api used by instances [rabbit-infra, rabbit-old]\n"+
			"  amqp_url amqp://rabbit:5672 used by instances [rabbit-infra, rabbit-old]\n"+
			"Leave one instance per URL, then restart maas-service.")

		version, err := migrations.Version(dbPg)
		require.NoError(t, err)
		assert.Equal(t, int64(31), version)
		assert.Equal(t, 0, countRabbitUrlIndexes(t, dbPg))

		// existing rows are unmanaged
		for _, table := range []string{"kafka_instances", "rabbit_instances"} {
			var unmanaged int
			_, err = dbPg.QueryOne(pgv10.Scan(&unmanaged), `SELECT count(*) FROM `+table+
				` WHERE managed_by_operator = false AND cr_namespace = '' AND cr_name = ''`)
			require.NoError(t, err)
			var total int
			_, err = dbPg.QueryOne(pgv10.Scan(&total), `SELECT count(*) FROM `+table)
			require.NoError(t, err)
			assert.Equal(t, total, unmanaged, table)
		}

		// an older maas-service does not know the new columns and must still be able to insert
		_, err = dbPg.Exec(`INSERT INTO kafka_instances (id, addresses, is_default) VALUES ('kafka-downgraded', '{"PLAINTEXT": ["kafka-2:9092"]}', false)`)
		assert.NoError(t, err)

		_, err = dbPg.Exec(`DELETE FROM rabbit_instances WHERE id = 'rabbit-infra'`)
		require.NoError(t, err)
		_, _, err = migrations.Run(dbPg, "up")
		require.NoError(t, err)
		assert.Equal(t, 2, countRabbitUrlIndexes(t, dbPg))

		_, err = dbPg.Exec(`INSERT INTO rabbit_instances (id, api_url, amqp_url, "user", password, is_default)
			VALUES ('rabbit-dup', 'http://rabbit:15672/api', 'amqp://dup:5672', 'u', 'p', false)`)
		assert.ErrorContains(t, err, RabbitApiUrlUniqueIndex)
		_, err = dbPg.Exec(`INSERT INTO rabbit_instances (id, api_url, amqp_url, "user", password, is_default)
			VALUES ('rabbit-dup', 'http://dup:15672/api', 'amqp://rabbit:5672', 'u', 'p', false)`)
		assert.ErrorContains(t, err, RabbitAmqpUrlUniqueIndex)

		// one broker per namespace: a second managed row of a kind in one CR namespace fails, unmanaged rows do not count
		for _, tc := range []struct{ table, owner, second, index string }{
			{"kafka_instances", "kafka-old", "kafka-downgraded", KafkaCrNamespaceUniqueIndex},
			{"rabbit_instances", "rabbit-old", "rabbit-other", RabbitCrNamespaceUniqueIndex},
		} {
			_, err = dbPg.Exec(`UPDATE `+tc.table+` SET managed_by_operator = true, cr_namespace = 'infra', cr_name = 'a' WHERE id = ?`, tc.owner)
			require.NoError(t, err, tc.table)
			_, err = dbPg.Exec(`UPDATE `+tc.table+` SET cr_namespace = 'infra', cr_name = 'b' WHERE id = ?`, tc.second)
			require.NoError(t, err, tc.table)
			_, err = dbPg.Exec(`UPDATE `+tc.table+` SET managed_by_operator = true WHERE id = ?`, tc.second)
			assert.ErrorContains(t, err, tc.index, tc.table)

			// Orphan of the owner frees the namespace
			_, err = dbPg.Exec(`UPDATE `+tc.table+` SET managed_by_operator = false, cr_namespace = '', cr_name = '' WHERE id = ?`, tc.owner)
			require.NoError(t, err, tc.table)
			_, err = dbPg.Exec(`UPDATE `+tc.table+` SET managed_by_operator = true WHERE id = ?`, tc.second)
			assert.NoError(t, err, tc.table)
		}
	})
}

func connect(tdb *testharness.TestDatabase) *pgv10.DB {
	return pgv10.Connect(&pgv10.Options{Addr: tdb.Addr(), User: tdb.Username(), Password: tdb.Password(), Database: tdb.DBName()})
}

func countRabbitUrlIndexes(t *testing.T, dbPg *pgv10.DB) int {
	var count int
	_, err := dbPg.QueryOne(pgv10.Scan(&count), `SELECT count(*) FROM pg_indexes WHERE tablename = 'rabbit_instances' AND indexname IN (?, ?)`,
		RabbitApiUrlUniqueIndex, RabbitAmqpUrlUniqueIndex)
	require.NoError(t, err)
	return count
}
