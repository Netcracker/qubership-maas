package main

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
)

type jwks struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	E   string `json:"e"`
	N   string `json:"n"`
}

func main() {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Panic(err)
	}
	e := base64Encode(strconv.Itoa(key.E))
	if err != nil {
		log.Panic(err)
	}
	n := base64Encode(key.N.String())
	if err != nil {
		log.Panic(err)
	}

	jwks := jwks{
		Keys: []jwk{
			{
				Kid: "first_key",
				Kty: "RSA",
				Use: "sig",
				Alg: "RS256",
				E:   string(e),
				N:   string(n),
			},
		},
	}
	openidConf := struct {
		JwksUri string `json:"jwks_uri"`
		Issuer  string `json:"issuer"`
	}{}

	err = http.ListenAndServe(":8080", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverUrl := fmt.Sprintf("http://%s", r.Host)
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			if openidConf.Issuer == "" {
				openidConf.Issuer = serverUrl
			}
			if openidConf.JwksUri == "" {
				openidConf.JwksUri = serverUrl + "/jwks"
			}
			openidConfJson, err := json.Marshal(openidConf)
			if err != nil {
				panic(err)
			}
			w.Write(openidConfJson)
		case "/jwks":
			rawJwks, err := json.Marshal(jwks)
			if err != nil {
				panic(err)
			}
			w.Write(rawJwks)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	if err != nil {
		log.Panic(err)
	}
}

func base64Encode(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}
