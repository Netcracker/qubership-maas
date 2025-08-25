package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	openid "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
)

const (
	tokenDir = "/var/run/secrets/kubernetes.io/serviceaccount"
	certPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
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

func NewVerifier(ctx context.Context, logger logging.Logger, issuer, audience string) (Verifier, error) {
	secureIssuer, err := isSecureIssuer(issuer)
	if err != nil {
		return nil, fmt.Errorf("failed to identify issuer: %w", err)
	}
	if secureIssuer {
		fts, err := newFileTokenSource(logger, tokenDir)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize a file token source: %w", err)
		}
		c, err := utils.NewSecureHttpClient(certPath, fts)
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
		return nil, errors.Join(err, msg.AuthError)
	}
	var claims Claims
	err = token.Claims(&claims)
	if err != nil {
		return nil, fmt.Errorf("required claims not present: %w: %w", err, msg.AuthError)
	}
	return &claims, nil
}

const (
	awsIssRegex       = `^oidc\.eks\..*\.amazonaws\.com$`
	gcpIssRegex       = `^container.googleapis.com$`
	localhostIssRegex = `^(localhost|127\.0\.0\.1)(:\d{1,5})?$`
)

func isSecureIssuer(issuer string) (bool, error) {
	awsPattern, err := regexp.Compile(awsIssRegex)
	if err != nil {
		return false, fmt.Errorf("failed to parse regexp pattern for issuer %s: %w", "AWS", err)
	}
	gcpPattern, err := regexp.Compile(gcpIssRegex)
	if err != nil {
		return false, fmt.Errorf("failed to parse regexp pattern for issuer %s: %w", "GCP", err)
	}
	localhostPattern, err := regexp.Compile(localhostIssRegex)
	if err != nil {
		return false, fmt.Errorf("failed to parse regexp pattern for issuer %s: %w", "GCP", err)
	}
	issuerUrl, err := url.Parse(issuer)
	if err != nil {
		return false, fmt.Errorf("invalid issuer url: %w", err)
	}
	switch {
	case awsPattern.MatchString(issuerUrl.Host):
		return false, nil
	case gcpPattern.MatchString(issuerUrl.Host):
		return false, nil
	case localhostPattern.MatchString(issuerUrl.Host):
		return false, nil
	default:
		return true, nil
	}
}
