package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#3")
	log.InfoC(ctx, "Register db evolution #3")
	migrations.DefaultCollection.DisableSQLAutodiscover(true)
	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table `rabbit_instances'...")
		if _, err := db.Exec(`create table rabbit_instances(
						"id" varchar not null,
						"api_url" varchar not null,
						"amqp_url" varchar not null,
						"user" varchar not null,
						"password" varchar not null,
						"is_default" boolean,
						CONSTRAINT pk_rabbit_instances PRIMARY KEY (id))
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table `rabbit_vhosts'...")
		if _, err := db.Exec(`create table rabbit_vhosts(
						"vhost" varchar not null,
						"user" varchar not null,
						"password" varchar not null,
						"namespace" varchar not null,
						"instance" varchar not null,
						"classifier" varchar not null)
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Create foreign key constraint on rabbit_vhosts instance column")
		if _, err := db.Exec(`alter table rabbit_vhosts
					add constraint fk_rabbit_vhosts
					foreign key (instance)
					references rabbit_instances (id)
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating hashed index on classifier...")
		if _, err := db.Exec(`create index idx_rabbit_vhosts_classifier_md5
						on rabbit_vhosts (
							decode(md5("classifier"), 'HEX')
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating unique index on classifier column...")
		if _, err := db.Exec(`create unique index rabbit_vhosts_classifier_uniq_idx
						on rabbit_vhosts (classifier)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #3 successfuly finished")
		return nil
	},

		func(db migrations.DB) error {
			log.InfoC(ctx, "dropping table `rabbit_instances'...")
			_, err := db.Exec(`DROP TABLE rabbit_instances`)

			if err != nil {
				return err
			}

			log.InfoC(ctx, "dropping table `rabbit_vhosts'...")
			_, err = db.Exec(`DROP TABLE rabbit_vhosts`)
			return err
		},
	)
}
