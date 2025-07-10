package encryption

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

var aesEncryptor = NewAes("thisis32bitlongpassphraseimusing")

func TestAes_Decrypt_Struct(t *testing.T) {
	secret := secretStruct{First: "firstValue", Second: 42}

	encryptedSecret, err := aesEncryptor.Encrypt(secret)
	assert.NoError(t, err)
	assert.NotEqual(t, secret, encryptedSecret)

	var decryptedSecret secretStruct
	err = aesEncryptor.Decrypt(encryptedSecret, &decryptedSecret)
	assert.NoError(t, err)
	assert.Equal(t, secret, decryptedSecret)
}
func TestAes_Decrypt_String(t *testing.T) {
	encryptedSecret, err := aesEncryptor.Encrypt("my-super-secret-value")
	assert.NoError(t, err)
	assert.NotEqual(t, "my-super-secret-value", encryptedSecret)

	var stringSecret string
	err = aesEncryptor.Decrypt(encryptedSecret, &stringSecret)
	assert.NoError(t, err)
	assert.Equal(t, "my-super-secret-value", stringSecret)

	encryptedSecret, err = aesEncryptor.Encrypt("")
	assert.NoError(t, err)
	assert.NotEqual(t, "", encryptedSecret)

	err = aesEncryptor.Decrypt(encryptedSecret, &stringSecret)
	assert.NoError(t, err)
	assert.Equal(t, "", stringSecret)
}
func TestAes_Decrypt_Nil(t *testing.T) {
	encryptedSecret, err := aesEncryptor.Encrypt(nil)
	assert.NoError(t, err)
	assert.NotNil(t, encryptedSecret)

	var decryptedVal interface{}
	err = aesEncryptor.Decrypt(encryptedSecret, &decryptedVal)
	assert.NoError(t, err)
	assert.Nil(t, decryptedVal)
}

func TestAes_Decrypt_Wrong_Decryption_Key(t *testing.T) {
	encryptedSecret, err := aesEncryptor.Encrypt("my-super-secret-value")
	assert.NoError(t, err)
	assert.NotNil(t, encryptedSecret)

	var stringSecret string
	wrongAesEncryptor := NewAes("_wrongwrongwrongwrongwrongwrong_")
	assert.PanicsWithValue(t, "malformed database cipher key", func() {
		_ = wrongAesEncryptor.Decrypt(encryptedSecret, &stringSecret)
	})
}

type secretStruct struct {
	First  string
	Second int
}
