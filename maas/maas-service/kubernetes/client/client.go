package client

import (
	"encoding/json"
	"io"
	"net/http"
)

const (
	tokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	certPath  = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

type Client struct {
	httpClient *http.Client
}

func New(secure bool) (*Client, error) {
	if secure {
		return newSecure()
	}
	return &Client{
		httpClient: http.DefaultClient,
	}, nil
}

func newSecure() (*Client, error) {
	httpClient, err := newSecureHttpClient(certPath, tokenPath)
	if err != nil {
		return nil, err
	}
	return &Client{
		httpClient: httpClient,
	}, nil
}

func (c *Client) GetOidcConfig(discoveryEndpoint string) (*OidcConfig, error) {
	resp, err := c.httpClient.Get(discoveryEndpoint)
	if err != nil {
		return nil, err
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var oidcConfig OidcConfig
	err = json.Unmarshal(respBody, &oidcConfig)
	if err != nil {
		return nil, err
	}
	return &oidcConfig, nil
}

func (c *Client) GetJwks(jwksEndpoint string) ([]byte, error) {
	resp, err := c.httpClient.Get(jwksEndpoint)
	if err != nil {
		return nil, err
	}
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return respBody, nil
}
