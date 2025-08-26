package oidc

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"

	openid "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/netcracker/qubership-maas/msg"
	"github.com/netcracker/qubership-maas/utils"
)

const (
	tokenDir = "/var/run/secrets/kubernetes.io/serviceaccount"
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

func NewVerifier(ctx context.Context, audience string) (Verifier, error) {
	fts, err := NewFileTokenSource(ctx, tokenDir)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize a file token source: %w", err)
	}
	c, err := utils.NewSecureHttpClient(fts)
	if err != nil {
		return nil, fmt.Errorf("failed to create secure http client: %w", err)
	}
	ctx = openid.ClientContext(ctx, c)

	issuer, err := getIssuer(fts)
	if err != nil {
		return nil, fmt.Errorf("failed to get issuer from the jwt token at dir %s: %w", tokenDir, err)
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
	token, err := jwt.ParseSigned(rawToken, []jose.SignatureAlgorithm{jose.RS256})
	if err != nil {
		return "", fmt.Errorf("invalid jwt: %w", err)
	}
	var claims Claims
	err = token.UnsafeClaimsWithoutVerification(claims)
	if err != nil {
		return "", fmt.Errorf("invalid jwt: %w", err)
	}
	if claims.Issuer == "" {
		return "", fmt.Errorf("jwt token does not have issuer value: %w", err)
	}
	return claims.Issuer, nil
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
