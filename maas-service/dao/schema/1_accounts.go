package schema

import (
	"github.com/go-pg/migrations/v8"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#1")
	log.InfoC(ctx, "Register db evolution #1")
	migrations.DefaultCollection.DisableSQLAutodiscover(true)
	migrations.MustRegisterTx(func(db migrations.DB) error {
		log.InfoC(ctx, "Creating table `accounts'...")
		if _, err := db.Exec(`create table accounts(
						"username" varchar(128) NOT NULL,
						"roles" jsonb NOT NULL,
						"salt" varchar(64) NOT NULL, 
						"password" text NOT NULL,
						"namespace" varchar(64) NOT NULL)
					`); err != nil {
			return err
		}

		log.InfoC(ctx, "Creating hashed index on username...")
		if _, err := db.Exec(`CREATE UNIQUE INDEX idx_accounts_username_and_namespace ON accounts (username)`); err != nil {
			return err
		}
		log.InfoC(ctx, "Evolution #1 successfuly finished")
		return nil
	})
}
