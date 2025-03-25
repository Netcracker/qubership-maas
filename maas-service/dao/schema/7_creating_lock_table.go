package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#7")
	log.InfoC(ctx, "Register db evolution #7")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating lock table for concurrent creating topic")
		if _, err := db.Exec(`CREATE TABLE locks("id" text PRIMARY KEY)`); err != nil {
			return err
		}
		log.InfoC(ctx, "Evolution #7 successfully finished")
		return nil
	})
}
