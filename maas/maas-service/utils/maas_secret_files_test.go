package utils

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPathHelpers(t *testing.T) {
	previous := MaasSecretsDir
	MaasSecretsDir = "/tmp/maas-secrets-test"
	defer func() { MaasSecretsDir = previous }()

	assert.Equal(t, filepath.Join(MaasSecretsDir, "maas-instance-registrations", "body"), PathInstanceRegistrations("body"))
	assert.Equal(t, filepath.Join(MaasSecretsDir, "maas-accounts", "manager-username"), PathMaasAccount("manager", "username"))
	assert.Equal(t, filepath.Join(MaasSecretsDir, "db", "pg_address"), PathDbSecret("pg_address"))
	assert.Equal(t, filepath.Join(MaasSecretsDir, "cipher", "key"), PathCipherSecret("key"))
}

func TestSecretDirMounted(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "file.txt")
	assert.NoError(t, os.WriteFile(filePath, []byte("x"), 0o600))

	assert.True(t, SecretDirMounted(tempDir))
	assert.False(t, SecretDirMounted(filePath))
	assert.False(t, SecretDirMounted(filepath.Join(tempDir, "missing")))
}

func TestReadFileWithProcessor(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	t.Run("missing file is skipped", func(t *testing.T) {
		called := false
		err := ReadFileWithProcessor(ctx, filepath.Join(tempDir, "missing"), func(body []byte) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("empty file is skipped", func(t *testing.T) {
		emptyFile := filepath.Join(tempDir, "empty")
		assert.NoError(t, os.WriteFile(emptyFile, []byte{}, 0o600))

		called := false
		err := ReadFileWithProcessor(ctx, emptyFile, func(body []byte) error {
			called = true
			return nil
		})
		assert.NoError(t, err)
		assert.False(t, called)
	})

	t.Run("processor receives content", func(t *testing.T) {
		secretFile := filepath.Join(tempDir, "secret")
		assert.NoError(t, os.WriteFile(secretFile, []byte("payload"), 0o600))

		called := false
		err := ReadFileWithProcessor(ctx, secretFile, func(body []byte) error {
			called = true
			assert.Equal(t, []byte("payload"), body)
			return nil
		})
		assert.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("processor error is returned", func(t *testing.T) {
		secretFile := filepath.Join(tempDir, "secret-error")
		assert.NoError(t, os.WriteFile(secretFile, []byte("payload"), 0o600))

		expectedErr := errors.New("processor failed")
		err := ReadFileWithProcessor(ctx, secretFile, func(body []byte) error {
			return expectedErr
		})
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("stat error is returned", func(t *testing.T) {
		err := ReadFileWithProcessor(ctx, "bad\x00path", func(body []byte) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error stating")
	})

	t.Run("read error is returned", func(t *testing.T) {
		err := ReadFileWithProcessor(ctx, tempDir, func(body []byte) error {
			return nil
		})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error reading")
	})
}

func TestReadSecretFromFileRequired(t *testing.T) {
	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "missing")

	value, err := ReadSecretFromFileRequired(missingPath)
	assert.Error(t, err)
	assert.Empty(t, value)
	assert.Contains(t, err.Error(), "required secret file missing")

	emptyPath := filepath.Join(tempDir, "empty")
	assert.NoError(t, os.WriteFile(emptyPath, []byte("  \n\t "), 0o600))
	value, err = ReadSecretFromFileRequired(emptyPath)
	assert.Error(t, err)
	assert.Empty(t, value)
	assert.Contains(t, err.Error(), "required secret file is empty")

	secretPath := filepath.Join(tempDir, "secret")
	assert.NoError(t, os.WriteFile(secretPath, []byte("  value \n"), 0o600))
	value, err = ReadSecretFromFileRequired(secretPath)
	assert.NoError(t, err)
	assert.Equal(t, "value", value)

	value, err = ReadSecretFromFileRequired(tempDir)
	assert.Error(t, err)
	assert.Empty(t, value)
	assert.Contains(t, err.Error(), "reading secret file")
}

func TestReadSecretBoolFromFileRequired(t *testing.T) {
	tempDir := t.TempDir()
	missingPath := filepath.Join(tempDir, "missing")

	value, err := ReadSecretBoolFromFileRequired(missingPath)
	assert.Error(t, err)
	assert.False(t, value)
	assert.Contains(t, err.Error(), "required secret file missing")

	emptyPath := filepath.Join(tempDir, "empty")
	assert.NoError(t, os.WriteFile(emptyPath, []byte("  \n\t "), 0o600))
	value, err = ReadSecretBoolFromFileRequired(emptyPath)
	assert.Error(t, err)
	assert.False(t, value)
	assert.Contains(t, err.Error(), "required secret file is empty")

	trueCases := []string{"true", "1", "yes", " TRUE "}
	for _, tc := range trueCases {
		boolPath := filepath.Join(tempDir, "bool-"+tc)
		assert.NoError(t, os.WriteFile(boolPath, []byte(tc), 0o600))
		value, err = ReadSecretBoolFromFileRequired(boolPath)
		assert.NoError(t, err)
		assert.True(t, value)
	}

	falsePath := filepath.Join(tempDir, "bool-false")
	assert.NoError(t, os.WriteFile(falsePath, []byte("false"), 0o600))
	value, err = ReadSecretBoolFromFileRequired(falsePath)
	assert.NoError(t, err)
	assert.False(t, value)

	value, err = ReadSecretBoolFromFileRequired(tempDir)
	assert.Error(t, err)
	assert.False(t, value)
	assert.Contains(t, err.Error(), "reading secret file")
}
