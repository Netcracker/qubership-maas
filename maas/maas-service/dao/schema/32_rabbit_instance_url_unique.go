package schema

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-maas/utils"
)

const (
	RabbitApiUrlUniqueIndex  = "uq_rabbit_instances_api_url"
	RabbitAmqpUrlUniqueIndex = "uq_rabbit_instances_amqp_url"
)

type sharedRabbitUrl struct {
	Column string `pg:"col"`
	Url    string `pg:"url"`
	Ids    string `pg:"ids"`
}

func init() {
	ctx := utils.CreateContextFromString("db-evo-#32")
	log.InfoC(ctx, "Register db evolution #32")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Checking that rabbit instance URLs are unique...")
		shared, err := findSharedRabbitUrls(db)
		if err != nil {
			return err
		}
		if len(shared) > 0 {
			return sharedRabbitUrlsError(shared)
		}

		log.InfoC(ctx, "Creating unique indexes on 'rabbit_instances' api_url and amqp_url...")
		if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ` + RabbitApiUrlUniqueIndex + ` ON rabbit_instances (api_url)`); err != nil {
			return err
		}
		if _, err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS ` + RabbitAmqpUrlUniqueIndex + ` ON rabbit_instances (amqp_url)`); err != nil {
			return err
		}

		log.InfoC(ctx, "Evolution #32 successfully finished")
		return nil
	})
}

func findSharedRabbitUrls(db migrations.DB) ([]sharedRabbitUrl, error) {
	var shared []sharedRabbitUrl
	_, err := db.Query(&shared, `
		SELECT 'api_url' AS col, api_url AS url, string_agg(id, ', ' ORDER BY id) AS ids
		FROM rabbit_instances GROUP BY api_url HAVING count(*) > 1
		UNION ALL
		SELECT 'amqp_url' AS col, amqp_url AS url, string_agg(id, ', ' ORDER BY id) AS ids
		FROM rabbit_instances GROUP BY amqp_url HAVING count(*) > 1
		ORDER BY col DESC, url`)
	return shared, err
}

func sharedRabbitUrlsError(shared []sharedRabbitUrl) error {
	var b strings.Builder
	b.WriteString("rabbit_instances: URLs shared by several instances, upgrade stopped:\n")
	for _, s := range shared {
		b.WriteString(fmt.Sprintf("  %-8s %s used by instances [%s]\n", s.Column, s.Url, s.Ids))
	}
	b.WriteString("Leave one instance per URL, then restart maas-service.")
	return errors.New(b.String())
}
