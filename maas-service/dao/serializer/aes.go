package serializer

import (
	"context"
	"fmt"
	"gorm.io/gorm/schema"
	"maas/maas-service/encryption"
	"maas/maas-service/utils"
	"reflect"
)

type Encrypted struct {
	encryptor *encryption.Aes
}

func NewEncrypted(encryptor *encryption.Aes) *Encrypted {
	return &Encrypted{
		encryptor: encryptor,
	}
}

func (e *Encrypted) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue interface{}) error {
	data, ok := dbValue.(string)
	if !ok {
		return fmt.Errorf("decryption only works on string data")
	}

	fieldValue := reflect.New(field.FieldType)
	err := e.encryptor.Decrypt(data, fieldValue.Interface())
	if err != nil {
		return utils.LogError(log, ctx, "can not deserialize encrypted value: %w", err)
	}
	field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())

	return nil
}

func (e *Encrypted) Value(_ context.Context, _ *schema.Field, _ reflect.Value, fieldValue interface{}) (interface{}, error) {
	return e.encryptor.Encrypt(fieldValue)
}
