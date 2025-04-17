package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#17")
	log.InfoC(ctx, "Register db evolution #17")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Updating rabbit_entities entity_name")
		if _, err := db.Exec("UPDATE rabbit_entities SET entity_name = NULL WHERE entity_name = ''"); err != nil {
			return err
		}

		log.InfoC(ctx, "Updating rabbit_entities binding_source")
		if _, err := db.Exec("UPDATE rabbit_entities SET binding_source = NULL WHERE binding_source = ''"); err != nil {
			return err
		}

		log.InfoC(ctx, "Updating rabbit_entities binding_destination")
		if _, err := db.Exec("UPDATE rabbit_entities SET binding_destination = NULL WHERE binding_destination = ''"); err != nil {
			return err
		}

		log.InfoC(ctx, "Add unique constraints on rabbit_entities table on vhost_id and entity_name")
		if _, err := db.Exec("ALTER TABLE rabbit_entities ADD UNIQUE (vhost_id,entity_type,entity_name);"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #17 successfully finished")
		return nil
	})
}
