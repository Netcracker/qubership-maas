package utils

import (
	"fmt"
	"net/http"
	"testing"
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
	validToken := "valid_token"
	client, err := NewSecureHttpClient(mockTokenSource{token: validToken})
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
