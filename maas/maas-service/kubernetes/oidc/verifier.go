package oidc

import (
	"context"
	"errors"
	"fmt"

	openid "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
)

var logger = logging.GetLogger("oidc")

type Claims struct {
	jwt.Claims
	Kubernetes K8sClaims `json:"kubernetes.io"`
}

type K8sClaims struct {
	Namespace      string         `json:"namespace,omitempty"`
	ServiceAccount ServiceAccount `json:"serviceaccount"`
}

type ServiceAccount struct {
	Name string `json:"name,omitempty"`
	Uid  string `json:"uid,omitempty"`
}

type Verifier interface {
	Verify(ctx context.Context, rawToken string) (*Claims, error)
}

type verifier struct {
	openidVerifier *openid.IDTokenVerifier
}

func NewVerifierDefault(ctx context.Context, audience string) (Verifier, error) {
	fts, err := NewFileTokenSourceDefault(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a file token source for oidcVerifier: %w", err)
	}
	return NewVerifier(ctx, audience, fts)
}

func NewVerifier(ctx context.Context, audience string, tokenSource utils.TokenSource) (Verifier, error) {
	c, err := utils.NewSecureHttpClient(tokenSource)
	if err != nil {
		return nil, fmt.Errorf("failed to create secure http client: %w", err)
	}
	ctx = openid.ClientContext(ctx, c)

	issuer, err := getIssuer(tokenSource)
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer from the jwt token: %w", err)
	}

	provider, err := openid.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to create oidc provider: %w", err)
	}
	v := provider.Verifier(&openid.Config{ClientID: audience})
	return &verifier{
		openidVerifier: v,
	}, nil
}

func (vf *verifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	token, err := vf.openidVerifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, errors.Join(err, msg.AuthError)
	}
	var claims Claims
	err = token.Claims(&claims)
	if err != nil {
		return nil, fmt.Errorf("required claims not present: %w: %w", err, msg.AuthError)
	}
	return &claims, nil
}

func getIssuer(ts utils.TokenSource) (string, error) {
	rawToken, err := ts.Token()
	if err != nil {
		return "", err
	}
	token, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256, "none"})
	if err != nil {
		return "", fmt.Errorf("invalid jwt: %w", err)
	}
	var claims Claims
	err = token.UnsafeClaimsWithoutVerification(&claims)
	if err != nil {
		return "", fmt.Errorf("invalid jwt: %w", err)
	}
	if claims.Issuer == "" {
		return "", fmt.Errorf("jwt token does not have issuer value: %w", err)
	}
	return claims.Issuer, nil
}
