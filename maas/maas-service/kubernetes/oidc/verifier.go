package oidc

import (
	"context"
	"fmt"

	openid "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4/jwt"
)

const (
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	certPath  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

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

func NewVerifier(ctx context.Context, secure bool, issuer, audience string) (Verifier, error) {
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
	v := provider.Verifier(&openid.Config{ClientID: audience})
	return &verifier{
		openidVerifier: v,
	}, nil
}

func (vf *verifier) Verify(ctx context.Context, rawToken string) (*Claims, error) {
	token, err := vf.openidVerifier.Verify(ctx, rawToken)
	if err != nil {
		return nil, err
	}
	var claims Claims
	err = token.Claims(&claims)
	if err != nil {
		return nil, fmt.Errorf("required claims not present: %w", err)
	}
	return &claims, nil
}
