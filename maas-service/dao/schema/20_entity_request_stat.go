package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#20")
	log.InfoC(ctx, "Register db evolution #20")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table entity_request_stat")
		_, err := db.Exec(`create table entity_request_stat(
						"namespace" varchar(100) NOT NULL,
						"microservice" varchar(100) NOT NULL,
						"entity_type" smallint,
						"name" varchar(255),
						"last_request_ts" timestamp NOT NULL DEFAULT NOW(),
						"requests_total" bigint NOT NULL ,
						PRIMARY KEY(namespace, microservice, entity_type, name)
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #20 successfully finished")
		return nil
	})
}
