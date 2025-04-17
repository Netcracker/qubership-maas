package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#11")
	log.InfoC(ctx, "Register db evolution #11")

	migrations.MustRegisterTx(func(db migrations.DB) error {

		log.InfoC(ctx, "Creating table 'instance_designators_kafka'...")
		if _, err := db.Exec(`create table instance_designators_kafka(
                        "id" SERIAL PRIMARY KEY,
						"namespace" text UNIQUE NOT NULL,
						"default_instance_id" text,
						   CONSTRAINT fk_instance_designators_kafka_instance_id
							  FOREIGN KEY(default_instance_id) 
							  REFERENCES kafka_instances(id)
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'instance_designators_kafka_selectors'...")
		if _, err := db.Exec(`create table instance_designators_kafka_selectors(
                        "id" SERIAL PRIMARY KEY,
						"instance_designator_id" integer NOT NULL,
						"instance_id" text NOT NULL,
						"classifier_match" jsonb NOT NULL,
						   CONSTRAINT fk_instance_designator_id
							  FOREIGN KEY(instance_designator_id) 
							  REFERENCES instance_designators_kafka(id) ON DELETE CASCADE,
						   CONSTRAINT fk_instance_designators_kafka_selectors_instance_id
							  FOREIGN KEY(instance_id) 
							  REFERENCES kafka_instances(id)
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'instance_designators_rabbit'...")
		if _, err := db.Exec(`create table instance_designators_rabbit(
                        "id" SERIAL PRIMARY KEY,
						"namespace" text UNIQUE NOT NULL,
						"default_instance_id" text,
						   CONSTRAINT fk_instance_designators_rabbit_instance_id
							  FOREIGN KEY(default_instance_id) 
							  REFERENCES rabbit_instances(id)
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating table 'instance_designators_rabbit_selectors'...")
		if _, err := db.Exec(`create table instance_designators_rabbit_selectors(
                        "id" SERIAL PRIMARY KEY,
						"instance_designator_id" integer NOT NULL,
						"instance_id" text NOT NULL,
						"classifier_match" jsonb NOT NULL,
						   CONSTRAINT fk_instance_designator_id
							  FOREIGN KEY(instance_designator_id) 
							  REFERENCES instance_designators_rabbit(id) ON DELETE CASCADE,
						   CONSTRAINT fk_instance_designators_rabbit_selectors_instance_id
							  FOREIGN KEY(instance_id) 
							  REFERENCES rabbit_instances(id)
						)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #11 successfully finished")
		return nil
	})
}
