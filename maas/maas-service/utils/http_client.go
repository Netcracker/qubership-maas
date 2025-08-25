package utils

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"time"
)

type TokenSource interface {
	Token() (string, error)
}

func NewSecureHttpClient(caCertPath string, tokenSource TokenSource) (*http.Client, error) {
	certPool, err := NewCertPool(caCertPath)
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
	return &http.Client{Transport: newSecureTransport(base, tokenSource)}, nil
}

func NewCertPool(certPath string) (*x509.CertPool, error) {
	pemCerts, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	pemBlocks, _ := pem.Decode(pemCerts)
	if pemBlocks == nil {
		return nil, fmt.Errorf("invalid ca cert file: %s", certPath)
	}
	certs, err := x509.ParseCertificates(pemBlocks.Bytes)
	if err != nil {
		return nil, fmt.Errorf("invalid ca cert file: %s", certPath)
	}
	certPool :=x509.NewCertPool()
	for _, cert := range certs {
		certPool.AddCert(cert)
	}
	return certPool, nil
}

type secureTransport struct {
	base http.RoundTripper
	ts   TokenSource
}

func newSecureTransport(base http.RoundTripper, ts TokenSource) *secureTransport {
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
