package oidc

import (
	"context"
	"fmt"

	openid "github.com/coreos/go-oidc/v3/oidc"
)

const (
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	certPath  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

func NewVerifier(ctx context.Context, secure bool, issuer, audience string) (*openid.IDTokenVerifier, error) {
	if secure {
		c, err := newSecureHttpClient(certPath, tokenPath)
		if err != nil {
			return nil, fmt.Errorf("failed to create secure http client: %w", err)
		}
		ctx = openid.ClientContext(ctx, c)
	}
	provider, err := openid.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create oidc provider: %w", err)
	}
	return provider.Verifier(&openid.Config{ClientID: audience}), nil
}
