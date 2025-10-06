package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#22")
	log.InfoC(ctx, "Register db evolution #22")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table bg_domains")
		_, err := db.Exec(`create table bg_domains(
    					"id" serial primary key,
						"origin_namespace" varchar(100) NOT NULL,
						"peer_namespace" varchar(100) NOT NULL,
						"created_when" timestamp default now()
						)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create bg_domains table: %w", err)
		}

		// todo refine sql error codes
		validatingTrigger := `
		create or replace function bg_domains_unique() returns trigger language plpgsql as $$
			declare
				c integer;
			begin
				if lower(new.origin_namespace) = lower(new.peer_namespace) then
					raise exception 'namespaces shouldn''t be the same: % <> %', new.origin_namespace, new.peer_namespace using errcode = '23505';
				end if;
		
				select count(*) into c from bg_domains where lower(new."origin_namespace") in (lower("origin_namespace"), lower("peer_namespace"));
				if (c > 0) then
					raise exception 'namespace "%" already used for other bg domain', new."origin_namespace" using errcode = '23505';
				end if;
		
				select count(*) into c from bg_domains where lower(new."peer_namespace") in (lower("origin_namespace"), lower("peer_namespace"));
				if (c > 0) then
					raise exception 'namespace "%" already used for other bg domain', new."peer_namespace" using errcode = '23505';
				end if;
		
			return new;
		end;
		$$`
		if _, err := db.Exec(validatingTrigger); err != nil {
			return utils.LogError(log, ctx, "error create validating trigger function: %w", err)
		}

		bindTriggerSql := `create trigger tr_bg_domains_unique before insert on bg_domains for each row execute procedure bg_domains_unique()`
		if _, err := db.Exec(bindTriggerSql); err != nil {
			return utils.LogError(log, ctx, "error insert table trigger: %w", err)
		}

		log.InfoC(ctx, "Creating table rabbit_vhost_classifiers")
		_, err = db.Exec(`create table rabbit_vhost_classifiers(
    					"id" serial primary key,
						"classifier" jsonb UNIQUE NOT NULL,
						"vhost_id" integer NOT NULL,
							CONSTRAINT fk_vhost_classifier_vhost_id
								FOREIGN KEY(vhost_id) 
							    REFERENCES rabbit_vhosts(id)
							ON DELETE CASCADE) `)
		if err != nil {
			return utils.LogError(log, ctx, "error create rabbit_vhost_classifiers table: %w", err)
		}

		log.InfoC(ctx, "Creating table bg_states")
		_, err = db.Exec(`create table bg_states(
    					"id" serial primary key,
    					"bg_domain_id" integer NOT NULL,
						"origin_ns" jsonb NOT NULL,
						"peer_ns" jsonb NOT NULL,
						"update_time" timestamp NOT NULL,
						   CONSTRAINT fk_bg_domain_id
							  FOREIGN KEY(bg_domain_id) 
							  REFERENCES bg_domains(id) ON DELETE CASCADE
						)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create bg_states table: %w", err)
		}

		log.InfoC(ctx, "Updating table accounts")
		_, err = db.Exec(`ALTER TABLE accounts ADD domain_namespaces text[]`)
		if err != nil {
			return utils.LogError(log, ctx, "error updating accounts table: %w", err)
		}

		log.InfoC(ctx, "Updating table kafka_topic_templates")
		_, err = db.Exec(`ALTER TABLE kafka_topic_templates ADD domain_namespaces text[]`)
		if err != nil {
			return utils.LogError(log, ctx, "error updating kafka_topic_templates table: %w", err)
		}

		log.InfoC(ctx, "Add numeric primary key to kafka_topics table")
		if _, err := db.Exec(`alter table kafka_topics add column id serial unique not null`); err != nil {
			return utils.LogError(log, ctx, "error create primary key for kafka_topics: %w", err)
		}

		log.InfoC(ctx, "Create table kafka_topic_classifiers")
		_, err = db.Exec(`create table kafka_topic_classifiers (
	id serial primary key, 
	topic_id integer not null, 
	classifier jsonb unique not null, 
	constraint fk_kafka_topic_classifiers foreign key(topic_id) references kafka_topics(id) on delete cascade)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create table kafka_topic_classifiers: %w", err)
		}
		_, err = db.Exec(`create index idx_kafka_topic_classifiers_topic_id on kafka_topic_classifiers using btree(topic_id)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create index idx_kafka_topic_classifiers_topic_id: %w", err)
		}

		log.InfoC(ctx, "Create table kafka_topic_definitions_classifiers")
		_, err = db.Exec(`create table kafka_topic_definitions_classifiers (
	id serial primary key, 
	topic_definition_id integer not null, 
	classifier jsonb unique not null, 
	constraint fk_kafka_topic_definitions_classifiers foreign key(topic_definition_id) references kafka_topic_definitions(id) on delete cascade)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create table kafka_topic_definitions_classifiers: %w", err)
		}
		_, err = db.Exec(`create index idx_kafka_topic_definitions_classifiers_topic_definition_id on kafka_topic_definitions_classifiers using btree(topic_definition_id)`)
		if err != nil {
			return utils.LogError(log, ctx, "error create index idx_kafka_topic_definitions_classifiers_topic_definition_id: %w", err)
		}

		log.InfoC(ctx, "Evolution #22 successfully finished")
		return nil
	})
}
