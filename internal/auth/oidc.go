package auth

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/runwisp/runwisp-manager/internal/config"
	"golang.org/x/oauth2"
)

type Provider struct {
	Config        config.Config
	OAuth         oauth2.Config
	UserinfoURL   string
	EndSessionURL string
	JWKSURL       string
	HTTP          *http.Client
	mu            sync.Mutex
	keys          map[string]json.RawMessage
	keysAt        time.Time
}

type Claims map[string]any

func Discover(ctx context.Context, c config.Config) (*Provider, error) {
	issuer := strings.TrimRight(c.OIDCIssuerURL, "/")
	if issuer == "" {
		return nil, errors.New("oidc issuer URL is required")
	}
	client, err := oidcHTTPClient(c)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, fmt.Errorf("create oidc discovery request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch oidc discovery: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc discovery returned HTTP %d", resp.StatusCode)
	}
	var metadata struct {
		Issuer                string `json:"issuer"`
		AuthorizationEndpoint string `json:"authorization_endpoint"`
		TokenEndpoint         string `json:"token_endpoint"`
		UserinfoEndpoint      string `json:"userinfo_endpoint"`
		JWKSURI               string `json:"jwks_uri"`
		EndSessionEndpoint    string `json:"end_session_endpoint"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&metadata); err != nil {
		return nil, fmt.Errorf("decode oidc discovery: %w", err)
	}
	if metadata.Issuer != issuer || metadata.AuthorizationEndpoint == "" || metadata.TokenEndpoint == "" || metadata.JWKSURI == "" {
		return nil, errors.New("oidc discovery metadata is incomplete or issuer does not match")
	}
	return &Provider{Config: c, OAuth: oauth2.Config{ClientID: c.OIDCClientID, ClientSecret: c.OIDCClientSecret, Endpoint: oauth2.Endpoint{AuthURL: metadata.AuthorizationEndpoint, TokenURL: metadata.TokenEndpoint}, RedirectURL: strings.TrimRight(c.ExternalURL, "/") + "/manager/auth/callback", Scopes: strings.Fields(c.OIDCScopes)}, UserinfoURL: metadata.UserinfoEndpoint, EndSessionURL: metadata.EndSessionEndpoint, JWKSURL: metadata.JWKSURI, HTTP: client, keys: make(map[string]json.RawMessage)}, nil
}

func (p *Provider) AuthCodeURL(state, nonce, verifier string, extra url.Values) string {
	options := []oauth2.AuthCodeOption{oauth2.SetAuthURLParam("code_challenge", codeChallenge(verifier)), oauth2.SetAuthURLParam("code_challenge_method", "S256"), oauth2.SetAuthURLParam("nonce", nonce)}
	for key, values := range extra {
		for _, value := range values {
			options = append(options, oauth2.SetAuthURLParam(key, value))
		}
	}
	return p.OAuth.AuthCodeURL(state, options...)
}

func (p *Provider) Exchange(ctx context.Context, code, verifier, nonce string) (Claims, string, error) {
	token, err := p.OAuth.Exchange(ctx, code, oauth2.SetAuthURLParam("code_verifier", verifier))
	if err != nil {
		return nil, "", fmt.Errorf("exchange oidc code: %w", err)
	}
	raw, ok := token.Extra("id_token").(string)
	if !ok || raw == "" {
		return nil, "", errors.New("oidc token response did not contain an id token")
	}
	claims, err := p.VerifyIDToken(ctx, raw, nonce)
	if err != nil {
		return nil, "", err
	}
	if p.Config.OIDCUserinfoClaims && p.UserinfoURL != "" {
		info, err := p.Userinfo(ctx, token.AccessToken)
		if err != nil {
			return nil, "", err
		}
		for key, value := range info {
			claims[key] = value
		}
	}
	return claims, raw, nil
}

func (p *Provider) Userinfo(ctx context.Context, accessToken string) (Claims, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.UserinfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create oidc userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch oidc userinfo: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc userinfo returned HTTP %d", resp.StatusCode)
	}
	var claims Claims
	if err := json.NewDecoder(resp.Body).Decode(&claims); err != nil {
		return nil, fmt.Errorf("decode oidc userinfo: %w", err)
	}
	return claims, nil
}

func (p *Provider) VerifyIDToken(ctx context.Context, raw, nonce string) (Claims, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid oidc id token")
	}
	decode := func(value string, out any) error {
		data, err := base64.RawURLEncoding.DecodeString(value)
		if err != nil {
			return err
		}
		return json.Unmarshal(data, out)
	}
	var header struct{ Algorithm, KeyID string }
	var claims Claims
	if err := decode(parts[0], &header); err != nil {
		return nil, errors.New("invalid oidc id token header")
	}
	if err := decode(parts[1], &claims); err != nil {
		return nil, errors.New("invalid oidc id token claims")
	}
	if err := p.verifySignature(ctx, header.Algorithm, header.KeyID, parts[0]+"."+parts[1], parts[2]); err != nil {
		return nil, err
	}
	if issuer, _ := claims["iss"].(string); issuer != strings.TrimRight(p.Config.OIDCIssuerURL, "/") {
		return nil, errors.New("oidc id token issuer mismatch")
	}
	if !audienceContains(claims["aud"], p.Config.OIDCClientID) {
		return nil, errors.New("oidc id token audience mismatch")
	}
	if tokenNonce, _ := claims["nonce"].(string); tokenNonce != nonce {
		return nil, errors.New("oidc id token nonce mismatch")
	}
	if exp, ok := number(claims["exp"]); !ok || time.Now().After(time.Unix(int64(exp), 0).Add(time.Minute)) {
		return nil, errors.New("oidc id token expired")
	}
	return claims, nil
}

func (p *Provider) verifySignature(ctx context.Context, algorithm, keyID, signing, encoded string) error {
	if algorithm != "RS256" && algorithm != "RS384" && algorithm != "RS512" && algorithm != "ES256" {
		return errors.New("unsupported oidc id token algorithm")
	}
	if err := p.loadKeys(ctx); err != nil {
		return err
	}
	keyData, ok := p.keys[keyID]
	if !ok {
		return errors.New("oidc signing key not found")
	}
	var key struct{ KTY, N, E, CRV, X, Y string }
	if err := json.Unmarshal(keyData, &key); err != nil {
		return errors.New("invalid oidc signing key")
	}
	sig, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return errors.New("invalid oidc id token signature")
	}
	data := []byte(signing)
	var hash crypto.Hash
	if strings.HasPrefix(algorithm, "RS") {
		hash = map[string]crypto.Hash{"RS256": crypto.SHA256, "RS384": crypto.SHA384, "RS512": crypto.SHA512}[algorithm]
		if !hash.Available() {
			return errors.New("oidc hash unavailable")
		}
		modulus, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return errors.New("invalid oidc rsa modulus")
		}
		exponent, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil || len(exponent) == 0 {
			return errors.New("invalid oidc rsa exponent")
		}
		e := 0
		for _, b := range exponent {
			e = e<<8 | int(b)
		}
		pub := &rsa.PublicKey{N: new(big.Int).SetBytes(modulus), E: e}
		digest := hash.New()
		digest.Write(data)
		if err := rsa.VerifyPKCS1v15(pub, hash, digest.Sum(nil), sig); err != nil {
			return errors.New("oidc id token signature mismatch")
		}
		return nil
	}
	if key.KTY != "EC" || key.CRV != "P-256" {
		return errors.New("invalid oidc ec signing key")
	}
	xb, err := base64.RawURLEncoding.DecodeString(key.X)
	if err != nil {
		return err
	}
	yb, err := base64.RawURLEncoding.DecodeString(key.Y)
	if err != nil {
		return err
	}
	pub := &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(xb), Y: new(big.Int).SetBytes(yb)}
	digest := sha256.Sum256(data)
	if len(sig) != 64 || !ecdsa.Verify(pub, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		return errors.New("oidc id token signature mismatch")
	}
	return nil
}

func (p *Provider) loadKeys(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if time.Since(p.keysAt) < time.Hour && len(p.keys) > 0 {
		return nil
	}
	// Discovery currently stores the JWKS endpoint in OAuth metadata only indirectly;
	// derive the standard endpoint for providers such as Pocket ID.
	endpoint := p.JWKSURL
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	resp, err := p.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("fetch oidc jwks: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("oidc jwks returned HTTP %d", resp.StatusCode)
	}
	var body struct {
		Keys []json.RawMessage `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("decode oidc jwks: %w", err)
	}
	p.keys = make(map[string]json.RawMessage)
	for _, item := range body.Keys {
		var meta struct {
			KID string `json:"kid"`
		}
		if json.Unmarshal(item, &meta) == nil && meta.KID != "" {
			p.keys[meta.KID] = item
		}
	}
	p.keysAt = time.Now()
	return nil
}

func oidcHTTPClient(c config.Config) (*http.Client, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.OIDCInsecureSkipTLSVerify} // #nosec G402 -- explicitly configured for development/provider quirks
	if c.OIDCCAFile != "" {
		data, err := os.ReadFile(c.OIDCCAFile)
		if err != nil {
			return nil, fmt.Errorf("read oidc CA: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, errors.New("parse oidc CA file")
		}
		tlsConfig.RootCAs = pool
	}
	return &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}, Timeout: 15 * time.Second}, nil
}

func audienceContains(value any, want string) bool {
	switch v := value.(type) {
	case string:
		return v == want
	case []any:
		for _, item := range v {
			if item == want {
				return true
			}
		}
	}
	return false
}
func number(value any) (float64, bool) { v, ok := value.(float64); return v, ok }
func codeChallenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}
