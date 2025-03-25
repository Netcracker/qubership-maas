package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#21")
	log.InfoC(ctx, "Register db evolution #21")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Update table entity_request_stat by adding instance field")
		_, err := db.Exec(`alter table entity_request_stat add column "instance" varchar(100)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #21 successfully finished")
		return nil
	})
}
