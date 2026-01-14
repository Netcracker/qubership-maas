package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#30")
	log.InfoC(ctx, "Register db evolution #30")

	migrations.MustRegisterTx(func(db migrations.DB) error {

		log.InfoC(ctx, "Creating table 'composite_namespace_modify_indexes'...")
		_, err := db.Exec(`create table if not exists composite_namespace_modify_indexes(
									 composite_namespace_id INTEGER PRIMARY KEY
										   REFERENCES composite_namespaces_v2(id)
										   ON DELETE CASCADE,
									 modify_index NUMERIC(20) NOT NULL CHECK (modify_index >= 0)
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #30 successfully finished")
		return nil
	})
}
