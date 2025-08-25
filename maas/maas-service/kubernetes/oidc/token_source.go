package oidc

import (
	"os"
	"sync"
	"time"
)

type fileTokenSource struct {
	mu     sync.RWMutex
	path   string
	token  string
	period time.Duration
	expiry time.Time
	now    func() time.Time
}

// if now func nil then time.Now is used
func newFileTokenSource(path string, now func() time.Time) *fileTokenSource {
	if now == nil {
		now = time.Now
	}
	return &fileTokenSource{
		path:   path,
		period: time.Minute,
		expiry: time.Now(),
		now:    now,
	}
}

func (f *fileTokenSource) Token() (string, error) {
	f.mu.RLock()
	if f.expiry.After(f.now()) {
		f.mu.RUnlock()
		return f.token, nil
	}
	f.mu.RUnlock()
	f.mu.Lock()
	defer f.mu.Unlock()
	tokenContents, err := os.ReadFile(f.path)
	if err != nil {
		return "", err
	}
	f.token = string(tokenContents)
	f.expiry = time.Now().Add(f.period)
	return f.token, nil
}
