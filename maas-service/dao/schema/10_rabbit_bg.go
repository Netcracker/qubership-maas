package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#10")
	log.InfoC(ctx, "Register db evolution #10")

	migrations.MustRegisterTx(func(db migrations.DB) error {

		log.InfoC(ctx, "Create primary key column on rabbit_vhosts")
		if _, err := db.Exec(`alter table rabbit_vhosts
					ADD COLUMN id SERIAL PRIMARY KEY
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'rabbit_ms_configs'...")
		if _, err := db.Exec(`create table rabbit_ms_configs(
                        "id" SERIAL PRIMARY KEY,
						"namespace" text,
						"ms_name" text, 
						"vhost_id" integer, 
						"candidate_version" text, 
						"actual_version" text, 
						"config" jsonb,
						   CONSTRAINT fk_vhost_id
							  FOREIGN KEY(vhost_id) 
							  REFERENCES rabbit_vhosts(id) ON DELETE CASCADE
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'rabbit_versioned_entities'...")
		if _, err := db.Exec(`create table rabbit_versioned_entities(
                        "id" SERIAL PRIMARY KEY,
						"ms_config_id" integer,
						"entity_type" text,
						"entity_name" text,
						"binding_source" text,
						"binding_destination" text,
						"client_entity" jsonb,
						"rabbit_entity" jsonb,
						   CONSTRAINT fk_ms_config_id
							  FOREIGN KEY(ms_config_id) 
							  REFERENCES rabbit_ms_configs(id) ON DELETE CASCADE
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating unique index on ms_name, vhost_id, candidate_version columns...")
		if _, err := db.Exec(`create unique index rabbit_ms_configs_username_ms_name_vhost_id_candidate_version_uniq_idx on rabbit_ms_configs (ms_name, vhost_id, candidate_version)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'bg_statuses'...")
		if _, err := db.Exec(`create table bg_statuses(
                        "id" SERIAL PRIMARY KEY,
						"namespace" text,
						"timestamp" timestamp DEFAULT CURRENT_TIMESTAMP,
						"active" text, 
						"legacy" text, 
						"candidates" text[]
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Drop not null constraints on kafka_topic_definitions table...")
		if _, err := db.Exec("ALTER TABLE kafka_topic_definitions ALTER COLUMN instance DROP NOT NULL"); err != nil {
			return err
		}

		if _, err := db.Exec("ALTER TABLE kafka_topic_definitions ALTER COLUMN name DROP NOT NULL"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #10 successfully finished")
		return nil
	})
}
