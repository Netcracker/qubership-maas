package oidc

import (
	"context"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/kubernetes/client"
)

type Provider struct {
	kuberClient  *client.Client
	jwtParser    *jwt.Parser
	keyFunc      jwt.Keyfunc
}

func NewProvider(ctx context.Context, log logging.Logger, client *client.Client, issuer, audience string) (*Provider, error) {
	oidcConf, err := client.GetOidcConfig(issuer + ".well-known/openid-configuration")
	if err != nil {
		return nil, fmt.Errorf("failed to get oidc config: issuer %s: %w", issuer, err)
	}
	jwtParser := jwt.NewParser(
		jwt.WithValidMethods(jwt.GetAlgorithms()),
		jwt.WithIssuedAt(),
		jwt.WithExpirationRequired(),
		jwt.WithIssuer(issuer),
		jwt.WithLeeway(time.Second*30),
		jwt.WithAudience(audience),
	)
	keyFunc, err := newKeyFunc(ctx, log, oidcConf.JwksUri)
	if err != nil {
		return nil, err
	}
	provider := Provider{
		kuberClient: client,
		jwtParser: jwtParser,
		keyFunc:     keyFunc,
	}
	return &provider, nil
}

func (p *Provider) Verify(token string) (*jwt.Token, error) {
	parsedToken, err := p.jwtParser.Parse(token, p.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("invalid jwt token: %w", err)
	}
	return parsedToken, nil
}
