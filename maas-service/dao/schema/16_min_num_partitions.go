package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#16")
	log.InfoC(ctx, "Register db evolution #16")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Alter table composite_namespaces")

		if _, err := db.Exec(`alter table kafka_topic_definitions ADD COLUMN min_num_partitions integer`); err != nil {
			return err
		}

		if _, err := db.Exec(`alter table kafka_topic_templates ADD COLUMN min_num_partitions integer`); err != nil {
			return err
		}

		if _, err := db.Exec(`alter table kafka_topics ADD COLUMN min_num_partitions integer`); err != nil {
			return err
		}
		log.InfoC(ctx, "Evolution #16 successfully finished")
		return nil
	})
}
