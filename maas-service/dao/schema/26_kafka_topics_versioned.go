package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#26")
	log.InfoC(ctx, "Register db evolution #26")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Adding column to kafka_topics table")
		if _, err := db.Exec(`alter table kafka_topics ADD COLUMN versioned bool default false`); err != nil {
			return err
		}
		log.InfoC(ctx, "Adding column to kafka_topic_definitions table")
		if _, err := db.Exec(`alter table kafka_topic_definitions ADD COLUMN versioned bool default false`); err != nil {
			return err
		}
		log.InfoC(ctx, "Adding column to kafka_topic_templates table")
		if _, err := db.Exec(`alter table kafka_topic_templates ADD COLUMN versioned bool default false`); err != nil {
			return err
		}
		log.InfoC(ctx, "Evolution #26 successfully finished")
		return nil
	})
}
