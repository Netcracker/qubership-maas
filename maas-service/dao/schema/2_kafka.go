package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#2")
	log.InfoC(ctx, "Register db evolution #2")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table `kafka_instances'...")
		_, err := db.Exec(`create table kafka_instances(
						"id" text PRIMARY KEY,
						"addresses" jsonb UNIQUE NOT NULL,
						"maas_protocol" text NOT NULL default 'PLAINTEXT',
						"ca_cert" text,
						"credentials" jsonb,
						"is_default" boolean)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table `kafka_topics'...")
		_, err = db.Exec(`create table kafka_topics(
						"classifier" text PRIMARY KEY,
						"topic" text NOT NULL,
						"instance" text NOT NULL references kafka_instances("id"),
						"namespace" text NOT NULL, 
						"num_partitions" integer, 
						"replication_factor" integer, 
						"replica_assignment" jsonb, 
						"configs" jsonb, 
						UNIQUE("topic", "instance"))`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Creating hashed index on classifier...")
		_, err = db.Exec(`create index idx_kafka_topics_classifier_md5 
						on kafka_topics (
							decode(md5("classifier"), 'HEX')
						)`)
		if err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #2 successfully finished")
		return nil
	})
}
