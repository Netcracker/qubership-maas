package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"testing"
	"time"
)

type mockTokenSource struct {
	token string
}

func (mt mockTokenSource) Token() (string, error) {
	return mt.token, nil
}

type mockRoundTripper struct {
	called bool
	token  string
}

func (m *mockRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	m.called = true
	if r.Header.Get("Authorization") != "Bearer "+m.token {
		return nil, fmt.Errorf("expected token %s, get %s", m.token, r.Header.Get("Authorization"))
	}
	return &http.Response{}, nil
}

func TestNewHttpClient(t *testing.T) {
	_, err := NewSecureHttpClient("not_existing_file", nil)
	if err == nil {
		t.Fatalf("expected non-nil error, got %v", err)
	}

	tempDir := t.TempDir()

	emptyCertFile, err := os.CreateTemp(tempDir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer emptyCertFile.Close()
	_, err = NewSecureHttpClient(emptyCertFile.Name(), nil)
	if err == nil {
		t.Fatalf("expected non-nil error, got %v", err)
	}

	caCert, err := createTestCaCert()
	if err != nil {
		t.Fatal(err)
	}
	caCertFile, err := os.CreateTemp(tempDir, "")
	if err != nil {
		t.Fatal(err)
	}
	defer caCertFile.Close()
	_, err = caCertFile.Write(caCert)
	if err != nil {
		t.Fatal(err)
	}

	validToken := "valid_token"
	client, err := NewSecureHttpClient(caCertFile.Name(), mockTokenSource{token: validToken})
	if err != nil {
		t.Fatalf("expected secure http client created succesfully, got err: %v", err)
	}
	transport := client.Transport.(*secureTransport)
	mockTransport := &mockRoundTripper{
		token: validToken,
	}
	transport.base = mockTransport

	_, err =client.Get("/test")
	if err != nil {
		t.Fatalf("expected nil err, got err: %v", err)
	}

	if !mockTransport.called {
		t.Fatalf("expected mockTransport to be called be the client")
	}
}

func createTestCaCert() ([]byte, error) {
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(2019),
		Subject: pkix.Name{
			Organization:  []string{"Test Company, INC."},
			Country:       []string{"Test Country"},
			Province:      []string{"Test Province"},
			Locality:      []string{"Test City"},
			StreetAddress: []string{"Test Address"},
			PostalCode:    []string{"74390"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0),
		IsCA:                  true,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, err
	}
	caBytes, err := x509.CreateCertificate(rand.Reader, ca, ca, &caPrivKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, err
	}
	caPem := new(bytes.Buffer)
	pem.Encode(caPem, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: caBytes,
	})
	return caPem.Bytes(), nil
}
