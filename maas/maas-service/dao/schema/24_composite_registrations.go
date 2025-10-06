package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
	"strings"
)

func init() {

	ctx := utils.CreateContextFromString("db-evo-#24")
	log.InfoC(ctx, "Register db evolution #24")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Update table composite_namespaces_v2")
		_, err := db.Exec(`create table composite_namespaces_v2(
                        "id" SERIAL PRIMARY KEY,
						"base_namespace" varchar(63) NOT NULL,
						"namespace" varchar(63) NOT NULL
						)`)

		_, err = db.Exec(`CREATE INDEX composite_namespaces_v2_base_namespace_idx ON composite_namespaces_v2(base_namespace)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`CREATE UNIQUE INDEX composite_namespaces_v2_pair_guard ON composite_namespaces_v2(base_namespace, namespace)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`CREATE UNIQUE INDEX composite_namespaces_v2_namespace ON composite_namespaces_v2(namespace)`)
		if err != nil {
			return err
		}

		_, err = db.Exec(`
			-- prevent insert baseline namespace without declaring in member column
			create or replace function composite_namespaces_v2_guard() 
				returns trigger language plpgsql as 
			$$
			begin 
				if exists (
					select distinct b1.base_namespace from 
						composite_namespaces_v2 b1 left join composite_namespaces_v2 b2 on b1.base_namespace = b2.namespace 
						where b2.namespace is null
					) then
					  raise exception 'baseline namespace "%" is not in member list', new.base_namespace using errcode = '23505';
				end if;
				return new;
			end;
			$$;

			create constraint trigger composite_namespaces_v2_guard
			  after insert on composite_namespaces_v2
			  deferrable initially deferred
			  for each row
			  execute procedure composite_namespaces_v2_guard();
			
			-- deny baseline namespace id change 
			create or replace function composite_namespaces_v2_base_namespace_deny_update() 
				returns trigger language plpgsql as 
			$$
			begin
				  raise exception 'composite id "%" can be changed after registration', new.namespace using errcode = '23505';
				return null; 
			end;
			$$;
			
			create trigger composite_namespaces_v2_base_namespace_deny_update
			  before update
			  on composite_namespaces_v2
			  for each row
			  execute procedure composite_namespaces_v2_base_namespace_deny_update();
		`)
		if err != nil {
			return err
		}

		var nss []namespace
		_, err = db.Query(&nss, `SELECT base_namespace, namespace FROM composite_namespaces`)
		if err != nil {
			return err
		}

		groupedByBaseline := make(map[string][]string)

		for _, n := range nss {
			groupedByBaseline[n.BaseNamespace] = append(groupedByBaseline[n.BaseNamespace], n.Namespace)
		}
		for baseline, ns := range groupedByBaseline {
			values := make([]any, 0)

			values = append(values, baseline, baseline)
			for _, n := range ns {
				if baseline == n {
					continue
				}
				values = append(values, baseline, n)
			}

			insertQuery := "INSERT INTO composite_namespaces_v2 (base_namespace, namespace) VALUES" + strings.Repeat(" (?, ?),", (len(values)/2)-1) + " (?, ?)"

			_, err = db.Exec(insertQuery, values...)
			if err != nil {
				return err
			}
		}

		log.InfoC(ctx, "Evolution #24 successfully finished")
		return nil
	})
}

type namespace struct {
	BaseNamespace string
	Namespace     string
}
