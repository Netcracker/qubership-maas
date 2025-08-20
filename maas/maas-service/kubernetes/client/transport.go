package client

import (
	"fmt"
	"net/http"
)

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
