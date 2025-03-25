package schema

import (
	"github.com/go-pg/migrations/v8"
	"github.com/netcracker/qubership-core-lib-go/v3/configloader"
	"maas/maas-service/encryption"
	"maas/maas-service/utils"
)

func init() {
	ctx := utils.CreateContextFromString("db-evo-#29")
	log.InfoC(ctx, "Register db evolution #29")

	migrations.MustRegisterTx(func(db migrations.DB) error {
		if _, err := db.Exec(`ALTER TABLE rabbit_instances ADD COLUMN password_enc text`); err != nil {
			return err
		}

		if _, err := db.Exec(`ALTER TABLE rabbit_instances ALTER COLUMN password DROP NOT NULL`); err != nil {
			return err
		}

		if _, err := db.Exec(`ALTER TABLE rabbit_instances ADD CONSTRAINT chk_password_password_enc CHECK ((password IS NOT NULL) OR (password_enc IS NOT NULL))`); err != nil {
			return err
		}

		var creds []rabbitInstanceCred
		_, err := db.Query(&creds, `SELECT id, password FROM rabbit_instances`)
		if err != nil {
			return err
		}

		cipherKey := configloader.GetKoanf().MustString("db.cipher.key")
		encryptor := encryption.NewAes(cipherKey)

		for _, c := range creds {
			encryptedCred, err := encryptor.Encrypt(c.Password)
			if err != nil {
				return err
			}
			insertQuery := "UPDATE rabbit_instances SET password_enc = ? WHERE id = ?"

			_, err = db.Exec(insertQuery, encryptedCred, c.Id)
			if err != nil {
				return err
			}
		}

		log.InfoC(ctx, "Evolution #29 successfully finished")
		return nil
	})
}

type rabbitInstanceCred struct {
	Id       string
	Password string
}
