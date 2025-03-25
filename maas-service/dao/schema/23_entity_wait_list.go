package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#23")
	log.InfoC(ctx, "Register db evolution #23")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table custom_resource_waits")
		_, err := db.Exec(`CREATE TABLE custom_resource_waits(
						"tracking_id" bigserial PRIMARY KEY,
						"custom_resource" jsonb NOT NULL,
						"namespace" varchar(63) NOT NULL,
						"reason" text NOT NULL,
						"status" varchar(32) NOT NULL, 
						"created_at" timestamp NOT NULL 
						)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`CREATE INDEX custom_resource_waits_namespace_idx ON custom_resource_waits (namespace)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`CREATE UNIQUE INDEX custom_resource_waits_custom_resource_name_namespace_idx ON 
			custom_resource_waits ((custom_resource->'Metadata'->>'Name'), (custom_resource->'Metadata'->>'Namespace'))`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #23 successfully finished")
		return nil
	})
}
