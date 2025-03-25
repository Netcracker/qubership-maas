package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#12")
	log.InfoC(ctx, "Register db evolution #12")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table composite_namespaces")
		_, err := db.Exec(`create table composite_namespaces(
                        "id" SERIAL PRIMARY KEY,
						"base_namespace" text NOT NULL,
						"namespace" text UNIQUE NOT NULL
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #12 successfully finished")
		return nil
	})
}
