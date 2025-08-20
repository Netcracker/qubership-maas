package oidc

import (
	"context"
	"time"

	"github.com/MicahParks/jwkset"
	"github.com/MicahParks/keyfunc/v3"
	"github.com/golang-jwt/jwt/v5"
	"github.com/netcracker/qubership-core-lib-go/v3/logging"
	"golang.org/x/time/rate"
)

func createJWKSHTTPClient(ctx context.Context, log logging.Logger, jwksUrl string) (jwkset.Storage, error) {
	refreshErrorHandler := func(ctx context.Context, err error) {
		log.ErrorC(ctx, "failed to refresh jwks for kubernetes oidc: url %s: %v", jwksUrl, err)
	}
	options := jwkset.HTTPClientStorageOptions{
		Ctx:                 ctx,
		RefreshErrorHandler: refreshErrorHandler,
		RefreshInterval:     time.Minute * 5,
	}
	storage, err := jwkset.NewStorageFromHTTP(jwksUrl, options)
	if err != nil {
		return nil, err
	}
	clientOptions := jwkset.HTTPClientOptions{
		HTTPURLs:          map[string]jwkset.Storage{jwksUrl: storage},
		RateLimitWaitMax:  time.Minute,
		RefreshUnknownKID: rate.NewLimiter(rate.Every(5*time.Minute), 1),
	}
	return jwkset.NewHTTPClient(clientOptions)
}

func newKeyFunc(ctx context.Context, log logging.Logger, jwksUrl string) (jwt.Keyfunc, error) {
	client, err := createJWKSHTTPClient(ctx, log, jwksUrl)
	if err != nil {
		return nil, err
	}
	keyFunc, err := keyfunc.New(keyfunc.Options{
		Ctx:     ctx,
		Storage: client,
	})
	if err != nil {
		return nil, err
	}
	return keyFunc.Keyfunc, nil
}
