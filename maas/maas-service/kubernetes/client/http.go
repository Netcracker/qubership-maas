package client

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

func newSecureHttpClient(certPath, tokenPath string) (*http.Client, error) {
	certPool, err := newCertPool(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create cert pool: %w", err)
	}
	base := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			RootCAs: certPool,
		},
	}
	return &http.Client{Transport: newSecureTransport(base, newFileTokenSource(tokenPath))}, nil
}
