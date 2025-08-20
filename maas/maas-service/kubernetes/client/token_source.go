package client

import (
	"os"
	"time"
)

type tokenSource interface {
	Token() (string, error)
}

type fileTokenSource struct {
	path   string
	token  string
	period time.Duration
	expiry time.Time
}

func newFileTokenSource(path string) *fileTokenSource {
	return &fileTokenSource{
		path:   path,
		period: time.Minute,
		expiry: time.Now(),
	}
}

func (f *fileTokenSource) Token() (string, error) {
	if f.expiry.After(time.Now()) {
		return f.token, nil
	}
	tokenContents, err := os.ReadFile(f.path)
	if err != nil {
		return "", err
	}
	f.token = string(tokenContents)
	f.expiry = time.Now().Add(f.period)
	return f.token, nil
}
