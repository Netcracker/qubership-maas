package utils

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var MaasSecretsDir = "/var/run/secrets/maas"

func PathInstanceRegistrations(fileName string) string {
	return filepath.Join(MaasSecretsDir, "maas-instance-registrations", fileName)
}

func PathMaasAccount(account, suffix string) string {
	return filepath.Join(MaasSecretsDir, "maas-accounts", account+"-"+suffix)
}

func PathDbSecret(key string) string {
	return filepath.Join(MaasSecretsDir, "db", key)
}

func PathCipherSecret(key string) string {
	return filepath.Join(MaasSecretsDir, "cipher", key)
}

// When true, DB/cipher secrets are read from files and are required; when false, config/env fallback is used.
func SecretDirMounted(dir string) bool {
	info, err := os.Stat(dir)
	return err == nil && info.IsDir()
}

// ReadFileWithProcessor reads path if it exists and is non-empty, then calls processor(contents).
// Instance registration and account files are optional: if the file does not exist or is empty, returns nil (skip).
func ReadFileWithProcessor(ctx context.Context, path string, processor func(body []byte) error) error {
	log.InfoC(ctx, "Check file: `%v'", path)
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		log.InfoC(ctx, "File `%s` is not exists, skip postdeploy action", path)
		return nil
	}
	if err != nil {
		return fmt.Errorf("error stating `%s': %w", path, err)
	}
	log.InfoC(ctx, "Load file: `%v'", path)
	contents, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading `%s': %w", path, err)
	}
	if len(contents) == 0 {
		log.InfoC(ctx, "File `%s' has empty body, skip processing", path)
		return nil
	}
	return processor(contents)
}

// ReadSecretFromFileRequired reads the secret from filePath. Returns error if the file is missing or empty.
// Use for DB and cipher secrets when they must be provided via file (no env fallback).
func ReadSecretFromFileRequired(filePath string) (string, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("required secret file missing: %s", filePath)
		}
		return "", fmt.Errorf("reading secret file %s: %w", filePath, err)
	}
	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", fmt.Errorf("required secret file is empty: %s", filePath)
	}
	return s, nil
}

func ReadSecretBoolFromFileRequired(filePath string) (bool, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, fmt.Errorf("required secret file missing: %s", filePath)
		}
		return false, fmt.Errorf("reading secret file %s: %w", filePath, err)
	}
	s := strings.TrimSpace(strings.ToLower(string(b)))
	if s == "" {
		return false, fmt.Errorf("required secret file is empty: %s", filePath)
	}
	return s == "true" || s == "1" || s == "yes", nil
}
