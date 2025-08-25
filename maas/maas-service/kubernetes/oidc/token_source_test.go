package oidc

import (
	"os"
	"sync"
	"testing"
	"time"

	"github.com/netcracker/qubership-core-lib-go/v3/logging"
)

var logger = logging.GetLogger("server")

func TestFileTokenSource(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	newToken := "new_token"
	stillValidToken := "still_valid_token"
	expiredToken := "expired_token"
	tests := []struct {
		name    string
		oldTok  string
		fTok    string
		expiry  time.Time
		now     func() time.Time
		wantTok string
	}{
		{
			name:    "token still valid. return old token",
			oldTok:  stillValidToken,
			expiry:  time.Now().Add(time.Minute),
			now:     func() time.Time { return time.Now() },
			wantTok: stillValidToken,
		},
		{
			name:    "token expired re read from file",
			oldTok:  expiredToken,
			fTok:    newToken,
			expiry:  time.Now(),
			now:     func() time.Time { return time.Now() },
			wantTok: newToken,
		},
	}
	for _, test := range tests {
		if test.fTok != "" {
			_, err := f.WriteAt([]byte(test.fTok), 0)
			if err != nil {
				t.Fatal(err)
			}
		}
		fts := newFileTokenSource(logger, f.Name(), test.now)
		fts.expiry = test.expiry
		fts.token = test.oldTok
		token, err := fts.Token()
		if err != nil {
			t.Fatal(err)
		}
		if test.wantTok != token {
			t.Errorf("test %q: expected token %s, got %s", test.name, test.wantTok, token)
		}
	}
}

func TestFileTokenSourceRace(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	fts := newFileTokenSource(logger, f.Name(), nil)
	var wg sync.WaitGroup
	errCh := make(chan error)
	done := make(chan bool)
	for range 10 {
		wg.Add(1)
		go func() {
			_, err := fts.Token()
			if err != nil {
				errCh <- err
			}
			wg.Done()
		}()
	}
	go func() { wg.Wait(); done <- true }()
	select {
	case err := <-errCh:
		t.Fatal(err)
	case <-done:
	}
}
