package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"maas/maas-service/encryption"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#28")
	log.InfoC(ctx, "Register db evolution #28")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		if _, err := db.Exec(`ALTER TABLE kafka_instances ADD COLUMN credentials_enc text`); err != nil {
			return err
		}

		var creds []kafkaInstanceCred
		_, err := db.Query(&creds, `SELECT id, credentials FROM kafka_instances`)
		if err != nil {
			return err
		}

		cipherKey := configloader.GetKoanf().MustString("db.cipher.key")
		encryptor := encryption.NewAes(cipherKey)

		for _, c := range creds {
			encryptedCred, err := encryptor.Encrypt(c.Credentials)
			if err != nil {
				return err
			}
			insertQuery := "UPDATE kafka_instances SET credentials_enc = ? WHERE id = ?"

			_, err = db.Exec(insertQuery, encryptedCred, c.Id)
			if err != nil {
				return err
			}
		}

		log.InfoC(ctx, "Evolution #28 successfully finished")
		return nil
	})
}

type kafkaInstanceCred struct {
	Id          string
	Credentials any
}
