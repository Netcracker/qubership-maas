package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#6")
	log.InfoC(ctx, "Register db evolution #6")

	migrations.MustRegisterTx(func(db migrations.DB) error {

		log.InfoC(ctx, "Changing 'kafka_topics' primary key on (classifier) with primary key on (classifier, instance, namespace,)...")
		log.InfoC(ctx, "Drop pkey constraint...")
		if _, err := db.Exec("ALTER TABLE kafka_topics DROP CONSTRAINT kafka_topics_pkey"); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating new primary key...")
		if _, err := db.Exec("ALTER TABLE kafka_topics ADD PRIMARY KEY (classifier, instance, namespace)"); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #6 successfully finished")
		return nil
	})
}
