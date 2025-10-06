package schema

import (
	"fmt"
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#14")
	log.InfoC(ctx, "Register db evolution #14")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating sync notify procedure...")
		if _, err := db.Exec(`create or replace function maas_cache_sync_notify() returns trigger language plpgsql
			as $function$
				declare 
					data json;
					notification json;
				begin
					notification = json_build_object(
									  'table', tg_table_name,
									  'action', tg_op,
									  'old', row_to_json(old),
									  'new', row_to_json(new));
					perform pg_notify('events', notification::text);
					return null; 
				end;
			$function$`); err != nil {
			return fmt.Errorf("migration #14 error: %w", err)
		}

		log.InfoC(ctx, "Evolution #14 successfully finished")
		return nil
	})
}
