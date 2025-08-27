package oidc_test

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	openid "github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"github.com/netcracker/qubership-maas/kubernetes/oidc"
)

const (
	aud       = "maas"
	sa        = "test-service-account"
	sub       = "system:serviceaccount:default:test-service-account"
	namespace = "default"
	uuid      = "test-uuid"
)

var logger = logging.GetLogger("server")

var tests = []struct {
	name   string
	claims oidc.Claims
	ok     bool
}{
	{
		name: "valid token",
		claims: oidc.Claims{
			Claims: jwt.Claims{
				Subject:   sub,
				Audience:  jwt.Audience{aud},
				Expiry:    jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now()),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Kubernetes: oidc.K8sClaims{
				Namespace: namespace,
				ServiceAccount: oidc.ServiceAccount{
					Name: sa,
					Uid:  uuid,
				},
			},
		},
		ok: true,
	},
	{
		name: "expired token",
		claims: oidc.Claims{
			Claims: jwt.Claims{
				Subject:   sub,
				Audience:  jwt.Audience{aud},
				Expiry:    jwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
				NotBefore: jwt.NewNumericDate(time.Now()),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Kubernetes: oidc.K8sClaims{
				Namespace: namespace,
				ServiceAccount: oidc.ServiceAccount{
					Name: sa,
					Uid:  uuid,
				},
			},
		},
		ok: false,
	},
	{
		name: "wrong audience",
		claims: oidc.Claims{
			Claims: jwt.Claims{
				Subject:   sub,
				Audience:  jwt.Audience{"some-other-aud"},
				Expiry:    jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now()),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Kubernetes: oidc.K8sClaims{
				Namespace: namespace,
				ServiceAccount: oidc.ServiceAccount{
					Name: sa,
					Uid:  uuid,
				},
			},
		},
		ok: false,
	},
	{
		name: "wrong issuer",
		claims: oidc.Claims{
			Claims: jwt.Claims{
				Subject:   sub,
				Issuer:    "https://accounts.google.com",
				Audience:  jwt.Audience{aud},
				Expiry:    jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
				NotBefore: jwt.NewNumericDate(time.Now()),
				IssuedAt:  jwt.NewNumericDate(time.Now()),
			},
			Kubernetes: oidc.K8sClaims{
				Namespace: namespace,
				ServiceAccount: oidc.ServiceAccount{
					Name: sa,
					Uid:  uuid,
				},
			},
		},
		ok: false,
	},
}

func TestVerifier(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{
		Algorithm: jose.RS256,
		Key:       key,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}

	var oidcClientToken string
	server, err := setupServer(key.Public(), &oidcClientToken)
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()

	clientToken, err := generateJwt(signer, oidc.Claims{
		Claims: jwt.Claims{
			Issuer:    server.URL,
			Subject:   "system:serviceaccount:default:default",
			Audience:  jwt.Audience{"kubernetes.default.svc"},
			Expiry:    jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
			NotBefore: jwt.NewNumericDate(time.Now()),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
		Kubernetes: oidc.K8sClaims{
			Namespace: "default",
			ServiceAccount: oidc.ServiceAccount{
				Name: "default",
				Uid:  "12345678-1234-1234-1234-1234567890ab",
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	oidcClientToken = clientToken

	ctx := openid.ClientContext(context.Background(), server.Client())

	tokenFile, err := os.Create(t.TempDir() + "/token")
	if err != nil {
		t.Fatal(err)
	}
	defer tokenFile.Close()
	_, err = tokenFile.Write([]byte(oidcClientToken))
	if err != nil {
		t.Fatal(err)
	}
	fts, err := oidc.NewFileTokenSource(ctx, path.Dir(tokenFile.Name()))
	if err != nil {
		t.Fatal(err)
	}

	v, err := oidc.NewVerifier(ctx, aud, fts)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range tests {
		if test.claims.Issuer == "" {
			test.claims.Issuer = server.URL
		}
		rawToken, err := generateJwt(signer, test.claims)
		if err != nil {
			t.Fatal(err)
		}
		claims, err := v.Verify(context.Background(), rawToken)
		if (err == nil) != test.ok {
			t.Errorf("test %q: expected valid token: %t, got %t: err: %v: claims: %+v", test.name, test.ok, err == nil, err, test.claims)
		} else if err == nil && test.claims.Kubernetes != claims.Kubernetes {
			t.Errorf("test %q: expected claims: %+v, got %+v", test.name, test.name, *claims)
		}
	}
}

func generateJwt(signer jose.Signer, claims oidc.Claims) (string, error) {
	return jwt.Signed(signer).Claims(claims).Serialize()
}

func setupServer(key crypto.PublicKey, clientToken *string) (*httptest.Server, error) {
	jwks := jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{{
			Key:       key,
			KeyID:     "key-1",
			Algorithm: string(jose.RS256),
			Use:       "sig",
		}},
	}
	rawJwks, err := json.Marshal(jwks)
	if err != nil {
		return nil, err
	}
	openidConf := struct {
		JwksUri string `json:"jwks_uri"`
		Issuer  string `json:"issuer"`
	}{}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+*clientToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			if openidConf.Issuer == "" {
				openidConf.Issuer = server.URL
			}
			if openidConf.JwksUri == "" {
				openidConf.JwksUri = server.URL + "/jwks"
			}
			openidConfJson, err := json.Marshal(openidConf)
			if err != nil {
				panic(err)
			}
			w.Write(openidConfJson)
		case "/jwks":
			w.Write(rawJwks)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	return server, nil
}
