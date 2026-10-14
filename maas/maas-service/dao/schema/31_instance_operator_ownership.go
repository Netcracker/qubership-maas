package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

const (
	KafkaCrNamespaceUniqueIndex  = "uq_kafka_instances_cr_namespace"
	RabbitCrNamespaceUniqueIndex = "uq_rabbit_instances_cr_namespace"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#31")
	log.InfoC(ctx, "Register db evolution #31")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		// Defaults keep existing rows unmanaged and let an older maas-service, which does not know these columns,
		// keep inserting rows after a downgrade.
		for _, table := range []struct{ name, crNamespaceIndex string }{
			{"kafka_instances", KafkaCrNamespaceUniqueIndex},
			{"rabbit_instances", RabbitCrNamespaceUniqueIndex},
		} {
			log.InfoC(ctx, "Adding operator ownership columns to '%s'...", table.name)
			if _, err := db.Exec(`ALTER TABLE ` + table.name + `
						ADD COLUMN IF NOT EXISTS managed_by_operator boolean NOT NULL DEFAULT false,
						ADD COLUMN IF NOT EXISTS cr_namespace text NOT NULL DEFAULT '',
						ADD COLUMN IF NOT EXISTS cr_name text NOT NULL DEFAULT ''`); err != nil {
				return err
			}
			// One broker per namespace: at most one operator-managed row per CR namespace and kind.
			if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ` + table.crNamespaceIndex + ` ON ` + table.name +
				` (cr_namespace) WHERE managed_by_operator`); err != nil {
				return err
			}
		}

		log.InfoC(ctx, "Evolution #31 successfully finished")
		return nil
	})
}
