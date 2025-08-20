package oidc

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
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

func newCertPool(certPath string) (*x509.CertPool, error) {
	certPool := x509.NewCertPool()
	pemCerts, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	if certPool.AppendCertsFromPEM(pemCerts) {
		return nil, fmt.Errorf("invalid ca cert file: %s", certPath)
	}
	return certPool, nil
}

type secureTransport struct {
	base http.RoundTripper
	ts   tokenSource
}

func newSecureTransport(base http.RoundTripper, ts tokenSource) *secureTransport {
	return &secureTransport{
		base: base,
		ts:   ts,
	}
}

func (s *secureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	token, err := s.ts.Token()
	if err != nil {
		return nil, fmt.Errorf("failed to get k8s sa token: %w", err)
	}
	r.Header.Add("Authorization", "Bearer "+token)
	return s.base.RoundTrip(r)
}
