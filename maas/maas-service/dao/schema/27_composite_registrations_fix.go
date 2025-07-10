package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#27")
	log.InfoC(ctx, "Register db evolution #27")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Update composite namespace guard trigger")
		_, err := db.Exec(`
			-- prevent insert baseline namespace without declaring in member column
			create or replace function composite_namespaces_v2_guard()
				returns trigger language plpgsql as
			$$
			begin
				if exists (
					select distinct b1.base_namespace, b2.base_namespace from composite_namespaces_v2 b1 left join
						(select base_namespace from composite_namespaces_v2 where base_namespace = "namespace") as b2 on b1.base_namespace = b2.base_namespace
						where b2.base_namespace is null
					) then
					  raise exception 'baseline namespace "%" is not in member list', new.base_namespace using errcode = '23505';
				end if;
				return new;
			end;
			$$;

			drop trigger composite_namespaces_v2_guard on composite_namespaces_v2;

			create constraint trigger composite_namespaces_v2_guard
			  after insert or delete on composite_namespaces_v2
			  deferrable initially deferred
			  for each row
			  execute procedure composite_namespaces_v2_guard();
		`)
		if err != nil {
			return err
		}
		log.InfoC(ctx, "Evolution #27 successfully finished")
		return nil
	})
}
