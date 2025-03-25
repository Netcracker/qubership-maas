package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#25")
	log.InfoC(ctx, "Register db evolution #25")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Adding columns to bg_domains table")
		if _, err := db.Exec(`alter table bg_domains 
						ADD COLUMN "controller_namespace" varchar(63);
						`); err != nil {
			return err
		}
		//todo make NOT NULL after migration

		validatingTrigger := `
		create or replace function bg_domains_unique() returns trigger language plpgsql as $$
			declare
				c integer;
			begin
				if lower(new.origin_namespace) = lower(new.peer_namespace) OR lower(new.origin_namespace) = lower(new.controller_namespace) OR lower(new.peer_namespace) = lower(new.controller_namespace) then
					raise exception 'namespaces shouldn''t be the same: origin %, peer %, controller %', new.origin_namespace, new.peer_namespace, new.controller_namespace using errcode = '23505';
				end if;
		
				select count(*) into c from bg_domains where lower(new."origin_namespace") in (lower("origin_namespace"), lower("peer_namespace"), lower("controller_namespace"));
				if (c > 0) then
					raise exception 'namespace "%" already used for other bg domain', new."origin_namespace" using errcode = '23505';
				end if;
		
				select count(*) into c from bg_domains where lower(new."peer_namespace") in (lower("origin_namespace"), lower("peer_namespace"), lower("controller_namespace"));
				if (c > 0) then
					raise exception 'namespace "%" already used for other bg domain', new."peer_namespace" using errcode = '23505';
				end if;				

                select count(*) into c from bg_domains where lower(new."controller_namespace") in (lower("origin_namespace"), lower("peer_namespace"), lower("controller_namespace"));
				if (c > 0) then
					raise exception 'namespace "%" already used for other bg domain', new."controller_namespace" using errcode = '23505';
				end if;
		
			return new;
		end;
		$$`
		if _, err := db.Exec(validatingTrigger); err != nil {
			return utils.LogError(log, ctx, "error create validating trigger function: %w", err)
		}

		dropTriggerSql := `DROP TRIGGER IF EXISTS tr_bg_domains_unique on bg_domains`
		if _, err := db.Exec(dropTriggerSql); err != nil {
			return utils.LogError(log, ctx, "error drop table trigger: %w", err)
		}

		bindTriggerSql := `create trigger tr_bg_domains_unique before insert on bg_domains for each row execute procedure bg_domains_unique()`
		if _, err := db.Exec(bindTriggerSql); err != nil {
			return utils.LogError(log, ctx, "error insert table trigger: %w", err)
		}

		log.InfoC(ctx, "Evolution #25 successfully finished")
		return nil
	})
}
