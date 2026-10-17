package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/runwisp/runwisp-manager/internal/config"
)

func TestDiscoverAndVerifyFakeOIDC(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	server := newTestServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/openid-configuration":
			writeTestJSON(w, map[string]string{"issuer": "http://placeholder", "authorization_endpoint": "http://placeholder/auth", "token_endpoint": "http://placeholder/token", "jwks_uri": "http://placeholder/jwks"})
		case "/jwks":
			writeTestJSON(w, map[string]any{"keys": []any{map[string]string{"kid": "test", "kty": "RSA", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})}}})
		default:
			http.NotFound(w, r)
		}
	}))
	if server == nil {
		t.Skip("environment does not permit loopback test listeners")
	}
	defer server.Close()
	// The discovery document must contain the exact public issuer.
	server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/.well-known/openid-configuration" {
			writeTestJSON(w, map[string]string{"issuer": server.URL, "authorization_endpoint": server.URL + "/auth", "token_endpoint": server.URL + "/token", "jwks_uri": server.URL + "/jwks"})
			return
		}
		if r.URL.Path == "/jwks" {
			writeTestJSON(w, map[string]any{"keys": []any{map[string]string{"kid": "test", "kty": "RSA", "alg": "RS256", "n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()), "e": base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1})}}})
			return
		}
		http.NotFound(w, r)
	})
	c := config.Config{ExternalURL: "https://manager.example", OIDCIssuerURL: server.URL, OIDCClientID: "client", OIDCClientSecret: "secret", OIDCScopes: "openid", OIDCClaimUsername: "preferred_username", OIDCClaimName: "name", OIDCClaimEmail: "email", OIDCClaimGroups: "groups"}
	provider, err := Discover(tContext(), c)
	if err != nil {
		t.Fatal(err)
	}
	valid := signTestJWT(t, key, server.URL, "client", "nonce", time.Now().Add(time.Hour))
	if _, err := provider.VerifyIDToken(tContext(), valid, "nonce"); err != nil {
		t.Fatalf("valid token: %v", err)
	}
	for _, test := range []struct {
		name, nonce string
		expiry      time.Time
		audience    string
	}{{name: "wrong nonce", nonce: "wrong", expiry: time.Now().Add(time.Hour), audience: "client"}, {name: "expired", nonce: "nonce", expiry: time.Now().Add(-time.Hour), audience: "client"}, {name: "wrong audience", nonce: "nonce", expiry: time.Now().Add(time.Hour), audience: "other"}} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := provider.VerifyIDToken(tContext(), signTestJWT(t, key, server.URL, test.audience, "nonce", test.expiry), test.nonce); err == nil {
				t.Fatal("expected verification failure")
			}
		})
	}
}

func signTestJWT(t *testing.T, key *rsa.PrivateKey, issuer, audience, nonce string, expiry time.Time) string {
	encode := func(value any) string {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return base64.RawURLEncoding.EncodeToString(data)
	}
	header := encode(map[string]string{"alg": "RS256", "kid": "test", "typ": "JWT"})
	claims := encode(map[string]any{"iss": issuer, "aud": audience, "nonce": nonce, "exp": expiry.Unix(), "sub": "sub"})
	input := header + "." + claims
	digest := sha256.Sum256([]byte(input))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return input + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func writeTestJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func tContext() context.Context { return context.Background() }

func newTestServer(handler http.Handler) (server *httptest.Server) {
	defer func() { _ = recover() }()
	return httptest.NewServer(handler)
}
