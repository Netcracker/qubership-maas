package schema

import (
	"fmt"
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#19")
	log.InfoC(ctx, "Register db evolution #19")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Add sync event payload table")
		if _, err := db.Exec("create table cache_sync_payload (id serial primary key, ts timestamp default now(), data json)"); err != nil {
			return fmt.Errorf("migration #19 error: error create cache_sync_payload table: %w", err)
		}

		// fix trigger written on migration #14
		if _, err := db.Exec(`create or replace function maas_cache_sync_notify() returns trigger language plpgsql
			as $function$
				declare 
					p_data json;
					p_id integer;
				begin
					p_data = json_build_object(
									  'table', tg_table_name,
									  'action', tg_op,
									  'old', row_to_json(old),
									  'new', row_to_json(new));
					
					delete from cache_sync_payload where ts <  now() - interval '10 minutes';
					insert into cache_sync_payload(data) values (p_data) returning id into p_id;

					perform pg_notify('sync', p_id::text);
					return null; 
				end;
			$function$`); err != nil {
			return fmt.Errorf("migration #19 error: error update trigger function: %w", err)
		}

		// exclude locks from sync
		if _, err := db.Exec(`drop trigger if exists locks_sync on locks`); err != nil {
			return fmt.Errorf("migration #19 error: error drop locks sync trigger: %w", err)
		}

		log.InfoC(ctx, "Evolution #19 successfully finished")
		return nil
	})
}
