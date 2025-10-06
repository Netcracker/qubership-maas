package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#13")
	log.InfoC(ctx, "Register db evolution #13")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		//todo set unique not on client_entity but on (vhost, client_entity)
		log.InfoC(ctx, "Creating table 'rabbit_entities'...")
		if _, err := db.Exec(`create table rabbit_entities(
                        "id" SERIAL PRIMARY KEY,
						"vhost_id" integer,
						"namespace" text,
						"classifier" text,
						"entity_type" text,
						"entity_name" text,
						"binding_source" text,
						"binding_destination" text,
						"client_entity" jsonb UNIQUE,
						"rabbit_entity" jsonb, 
						   CONSTRAINT fk_vhost_id_rabbit_entities
							  FOREIGN KEY(vhost_id) 
							  REFERENCES rabbit_vhosts(id) ON DELETE CASCADE
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #13 successfully finished")
		return nil
	})
}
