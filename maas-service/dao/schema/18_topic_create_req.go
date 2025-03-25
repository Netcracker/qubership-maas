package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#18")
	log.InfoC(ctx, "Register db evolution #18")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Add kafka_topics create_req")
		if _, err := db.Exec("ALTER TABLE kafka_topics ADD create_req json DEFAULT null"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #18 successfully finished")
		return nil
	})
}
