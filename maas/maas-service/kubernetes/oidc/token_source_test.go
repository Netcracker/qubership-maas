package oidc

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestFileTokenSource(t *testing.T) {
	tokenDir := t.TempDir()
	tokenFilePath := tokenDir + "/token"
	dataSymlinkPath := tokenDir + "/..data"
	tokenFile, err := os.CreateTemp(tokenDir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenFile.Close()
	err = os.Symlink(tokenFile.Name(), dataSymlinkPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(dataSymlinkPath, tokenFilePath)
	if err != nil {
		t.Fatal(err)
	}

	firstValidToken := "first_valid_token"
	_, err = tokenFile.Write([]byte(firstValidToken))
	if err != nil {
		t.Fatal(err)
	}

	fts, err := NewFileTokenSource(context.Background(), tokenDir)
	if err != nil {
		t.Fatal(err)
	}
	token, err := fts.Token()
	if err != nil {
		t.Fatal(err)
	}
	if firstValidToken != token {
		t.Errorf("expected token %s, got %s", firstValidToken, token)
	}

	secondValidToken := "second_valid_token"
	_, err = tokenFile.WriteAt([]byte(secondValidToken), 0)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Remove(dataSymlinkPath)
	if err != nil {
		t.Fatal(err)
	}
	err = os.Symlink(tokenFile.Name(), dataSymlinkPath)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Millisecond*50)
	token, err = fts.Token()
	if err != nil {
		t.Fatal(err)
	}
	if secondValidToken != token {
		t.Errorf("expected token %s, got %s", secondValidToken, token)
	}
}
