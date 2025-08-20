package client

import (
	"crypto/x509"
	"fmt"
	"os"
)

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
