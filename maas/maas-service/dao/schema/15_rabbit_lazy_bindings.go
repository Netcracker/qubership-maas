package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#15")
	log.InfoC(ctx, "Register db evolution #15")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Delete rows with null vhost_id or namespace")
		if _, err := db.Exec("DELETE FROM rabbit_entities WHERE namespace IS NULL OR vhost_id IS NULL"); err != nil {
			return err
		}

		log.InfoC(ctx, "Deleting properties_key from client_entity json")
		if _, err := db.Exec("UPDATE rabbit_entities SET client_entity = client_entity - 'properties_key' WHERE client_entity ->> 'properties_key' IS NOT NULL"); err != nil {
			return err
		}

		log.InfoC(ctx, "Add not null constraints on rabbit_entities table on vhost_id")
		if _, err := db.Exec("ALTER TABLE rabbit_entities ALTER COLUMN vhost_id set NOT NULL"); err != nil {
			return err
		}

		log.InfoC(ctx, "Add not null constraints on rabbit_entities table on namespace")
		if _, err := db.Exec("ALTER TABLE rabbit_entities ALTER COLUMN namespace set NOT NULL"); err != nil {
			return err
		}

		log.InfoC(ctx, "Drop unique constraints on rabbit_entities table on client_entity")
		if _, err := db.Exec("ALTER TABLE rabbit_entities DROP CONSTRAINT rabbit_entities_client_entity_key"); err != nil {
			return err
		}

		log.InfoC(ctx, "Add unique constraints on rabbit_entities table on namespace and client_entity")
		if _, err := db.Exec("ALTER TABLE rabbit_entities ADD UNIQUE (vhost_id, client_entity)"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #15 successfully finished")
		return nil
	})
}
