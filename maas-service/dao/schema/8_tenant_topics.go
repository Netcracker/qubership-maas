package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#8")
	log.InfoC(ctx, "Register db evolution #8")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Adding column 'kind' to lazy topic table...")
		if _, err := db.Exec(`alter table kafka_lazy_topics ADD COLUMN "kind" text`); err != nil {
			return err
		}

		log.InfoC(ctx, "migrating kind of previous lazy topics to 'lazy'...")
		if _, err := db.Exec(`UPDATE kafka_lazy_topics SET kind = 'lazy'`); err != nil {
			return err
		}

		log.InfoC(ctx, "renaming lazy topics to topic definitions...")
		if _, err := db.Exec(`ALTER TABLE kafka_lazy_topics RENAME TO kafka_topic_definitions;`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'tenants'...")
		_, err := db.Exec(`create table tenants(
                        "id" SERIAL PRIMARY KEY, 
						"namespace" text, 
						"external_id" text, 
						"tenant_presentation" jsonb
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Creating unique index on username and external_id columns...")
		if _, err := db.Exec(`create unique index tenants_username_external_id_uniq_idx on tenants (namespace, external_id)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Adding column 'tenant_id' to kafka_topics table...")
		if _, err := db.Exec(`alter table kafka_topics ADD COLUMN "tenant_id" integer`); err != nil {
			return err
		}

		log.InfoC(ctx, "Create foreign key constraint on kafka_topics")
		if _, err := db.Exec(`alter table kafka_topics
					add constraint fk_tenant_id
					foreign key (tenant_id)
					references tenants (id)
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #8 successfully finished")
		return nil
	})
}
