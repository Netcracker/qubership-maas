package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#5")
	log.InfoC(ctx, "Register db evolution #5")

	migrations.MustRegisterTx(func(db migrations.DB) error {

		log.InfoC(ctx, "Add column externally_managed...")
		_, err := db.Exec(`alter table kafka_topics add column externally_managed boolean default false`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #5 successfully finished")
		return nil
	})
}
