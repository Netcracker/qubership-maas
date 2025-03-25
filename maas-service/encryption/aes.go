package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
)

type Aes struct {
	key string
}

func NewAes(key string) *Aes {
	return &Aes{
		key: key,
	}
}

func (s *Aes) Decrypt(encryptedValue string, dst any) error {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedValue)
	if err != nil {
		return err
	}

	block, err := aes.NewCipher([]byte(s.key))
	if err != nil {
		return err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return errors.New("malformed ciphertext")
	}

	dec, err := gcm.Open(nil,
		ciphertext[:gcm.NonceSize()],
		ciphertext[gcm.NonceSize():],
		nil,
	)

	if dec == nil {
		log.Panic("malformed database cipher key")
	}

	return json.Unmarshal(dec, dst)
}

func (s *Aes) Encrypt(plainValue any) (string, error) {
	block, err := aes.NewCipher([]byte(s.key))
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	_, err = io.ReadFull(rand.Reader, nonce)
	if err != nil {
		return "", err
	}

	jsonValue, err := json.Marshal(plainValue)
	if err != nil {
		return "", err
	}

	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, jsonValue, nil)), nil
}
