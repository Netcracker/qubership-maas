package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#4")
	log.InfoC(ctx, "Register db evolution #4")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table `kafka_topic_templates'...")
		_, err := db.Exec(`create table kafka_topic_templates(
						"id" SERIAL PRIMARY KEY, 
						"name" text NOT NULL, 
						"namespace" text NOT NULL, 
						"num_partitions" integer, 
						"replication_factor" integer, 
						"replica_assignment" jsonb, 
						"configs" jsonb
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Creating unique index on name and namespace columns...")
		if _, err := db.Exec(`create unique index kafka_topic_templates_name_namespace_uniq_idx
						on kafka_topic_templates (name, namespace)`); err != nil {
			return err
		}

		//TODO: dirty state - when template has started to apply, but haven't reached particular topic. Then it is dirty, need postprocessor to apply it to dirty topics.
		log.InfoC(ctx, "Adding columns to topic table...")
		if _, err := db.Exec(`alter table kafka_topics 
						ADD COLUMN "template" integer, 
						ADD COLUMN "dirty" boolean DEFAULT FALSE,
						ADD CONSTRAINT fk_topic_template 
						FOREIGN KEY (template) 
						REFERENCES kafka_topic_templates (id);
						`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'kafka_lazy_topics'...")
		if _, err := db.Exec(`create table kafka_lazy_topics(
						"id" SERIAL PRIMARY KEY,
						"classifier" text NOT NULL,
						"namespace" text NOT NULL,
						"name" text NOT NULL,
						"instance" text NOT NULL,
						"num_partitions" integer, 
						"replication_factor" integer, 
						"replica_assignment" jsonb, 
						"configs" jsonb, 
						"template" text
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating unique index on classifier and namespace columns...")
		if _, err := db.Exec(`create unique index kafka_lazy_topics_classifier_namespace_uniq_idx
						on kafka_lazy_topics (classifier, namespace)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #4 successfully finished")
		return nil
	})
}
