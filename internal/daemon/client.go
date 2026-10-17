// SPDX-License-Identifier: GPL-3.0-or-later
// Package daemon provides the small typed client used by the manager.
package daemon

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/runwisp/runwisp-manager/internal/chap"
	"github.com/runwisp/runwisp-manager/internal/config"
)

type Transport string

const (
	TransportUnix Transport = "unix"
	TransportHTTP Transport = "http"
)

type Client interface {
	Transport() Transport
	RoundTripper() http.RoundTripper
	BaseURL() *url.URL
	Info(context.Context) (DaemonInfo, error)
	Identity(context.Context) (Identity, error)
	Tasks(context.Context) ([]Task, error)
	Reload(context.Context) (ReloadResult, error)
	Do(context.Context, string, string, any, any) error
}
type DaemonInfo struct {
	Version     string `json:"version"`
	Fingerprint string `json:"fingerprint"`
	StartedAt   string `json:"startedAt"`
	ConfigStale bool   `json:"configStale"`
}
type Identity struct {
	ConfigPath   string `json:"configPath"`
	DataDir      string `json:"dataDir"`
	ConfigSource string `json:"configSource"`
	PID          int    `json:"pid"`
	Version      string `json:"version"`
	Fingerprint  string `json:"fingerprint"`
}
type Task struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}
type ReloadResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type HTTPClient struct {
	base       *url.URL
	client     *http.Client
	password   string
	logger     *slog.Logger
	mu         sync.Mutex
	token      string
	loggedIn   time.Time
	readOnly   bool
	retryAfter time.Time
}
type UnixClient struct {
	base   *url.URL
	client *http.Client
}

func New(c config.Config, logger *slog.Logger) (Client, error) {
	if c.RunWispURL != "" {
		return newHTTP(c, logger)
	}
	socket := c.SocketPath()
	transport := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "unix", socket)
		if err != nil && errors.Is(err, os.ErrPermission) {
			return nil, fmt.Errorf("socket exists but connection refused: manager cannot access %s: %w", socket, err)
		}
		return conn, err
	}}
	return &UnixClient{base: &url.URL{Scheme: "http", Host: "runwisp.invalid"}, client: &http.Client{Transport: transport}}, nil
}

func newHTTP(c config.Config, logger *slog.Logger) (*HTTPClient, error) {
	base, err := url.Parse(c.RunWispURL)
	if err != nil {
		return nil, fmt.Errorf("parse RunWisp URL: %w", err)
	}
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: c.RunWispInsecureSkipVerify} // #nosec G402 -- explicitly configured for self-signed daemon certificates
	if c.RunWispCAFile != "" {
		data, err := os.ReadFile(c.RunWispCAFile)
		if err != nil {
			return nil, fmt.Errorf("read RunWisp CA: %w", err)
		}
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("parse RunWisp CA file")
		}
		tlsConfig.RootCAs = pool
	}
	return &HTTPClient{base: base, password: c.RunWispPassword, logger: logger, client: &http.Client{Transport: &http.Transport{TLSClientConfig: tlsConfig}}}, nil
}

func (c *UnixClient) Transport() Transport            { return TransportUnix }
func (c *UnixClient) RoundTripper() http.RoundTripper { return c.client.Transport }
func (c *UnixClient) BaseURL() *url.URL               { return c.base }
func (c *HTTPClient) Transport() Transport            { return TransportHTTP }
func (c *HTTPClient) RoundTripper() http.RoundTripper { return c.client.Transport }
func (c *HTTPClient) BaseURL() *url.URL               { return c.base }
func (c *UnixClient) Info(ctx context.Context) (DaemonInfo, error) {
	var out DaemonInfo
	return out, c.Do(ctx, http.MethodGet, "/api/daemon", nil, &out)
}
func (c *HTTPClient) Info(ctx context.Context) (DaemonInfo, error) {
	var out DaemonInfo
	return out, c.Do(ctx, http.MethodGet, "/api/daemon", nil, &out)
}
func (c *UnixClient) Identity(ctx context.Context) (Identity, error) {
	var out Identity
	return out, c.Do(ctx, http.MethodGet, "/api/daemon/identity", nil, &out)
}
func (c *HTTPClient) Identity(context.Context) (Identity, error) {
	return Identity{}, fmt.Errorf("daemon identity is only available over Unix transport")
}
func (c *UnixClient) Tasks(ctx context.Context) ([]Task, error) {
	var out []Task
	return out, c.Do(ctx, http.MethodGet, "/api/tasks", nil, &out)
}
func (c *HTTPClient) Tasks(ctx context.Context) ([]Task, error) {
	var out []Task
	return out, c.Do(ctx, http.MethodGet, "/api/tasks", nil, &out)
}
func (c *UnixClient) Reload(ctx context.Context) (ReloadResult, error) {
	var out ReloadResult
	return out, c.Do(ctx, http.MethodPost, "/api/daemon/reload", nil, &out)
}
func (c *HTTPClient) Reload(ctx context.Context) (ReloadResult, error) {
	var out ReloadResult
	return out, c.Do(ctx, http.MethodPost, "/api/daemon/reload", nil, &out)
}
func (c *UnixClient) Do(ctx context.Context, method, path string, body any, out any) error {
	return doJSON(ctx, c.client, c.base, method, path, body, out, "")
}
func (c *HTTPClient) Do(ctx context.Context, method, path string, body any, out any) error {
	for attempt := 0; attempt < 2; attempt++ {
		if err := c.loginIfNeeded(ctx); err != nil {
			return err
		}
		err := doJSON(ctx, c.client, c.base, method, path, body, out, c.token)
		if !errors.Is(err, errUnauthorized) {
			return err
		}
		c.mu.Lock()
		c.token = ""
		c.mu.Unlock()
	}
	return fmt.Errorf("daemon authentication failed after retry")
}

var errUnauthorized = errors.New("daemon returned unauthorized")

func (c *HTTPClient) loginIfNeeded(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Since(c.loggedIn) < 23*time.Hour {
		return nil
	}
	var challenge struct {
		Nonce string `json:"nonce"`
	}
	if err := doJSON(ctx, c.client, c.base, http.MethodGet, "/api/auth/challenge", nil, &challenge, ""); err != nil {
		return fmt.Errorf("get daemon challenge: %w", err)
	}
	payload := map[string]string{"nonce": challenge.Nonce, "response": chap.Response(c.password, challenge.Nonce)}
	var login struct {
		Token    string `json:"token"`
		ReadOnly bool   `json:"readOnly"`
	}
	if err := doJSON(ctx, c.client, c.base, http.MethodPost, "/api/auth/login", payload, &login, ""); err != nil {
		return fmt.Errorf("login to daemon: %w", err)
	}
	c.token, c.loggedIn, c.readOnly = login.Token, time.Now(), login.ReadOnly
	return nil
}

func doJSON(ctx context.Context, client *http.Client, base *url.URL, method, path string, body any, out any, token string) error {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode daemon request: %w", err)
		}
		reader = strings.NewReader(string(data))
	}
	target := *base
	target.Path = strings.TrimRight(base.Path, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, target.String(), reader)
	if err != nil {
		return fmt.Errorf("create daemon request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("daemon request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		wait := parseRetryAfter(resp.Header.Get("Retry-After"))
		if wait > 0 {
			select {
			case <-time.After(wait):
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return fmt.Errorf("daemon rate limited: retry after %s", wait)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
		return fmt.Errorf("daemon returned %s: %s", resp.Status, strings.TrimSpace(string(data)))
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode daemon response: %w", err)
	}
	return nil
}
func parseRetryAfter(value string) time.Duration {
	seconds, err := strconv.Atoi(value)
	if err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return 0
}
func configSocketPath(c config.Config) string { return filepath.Clean(c.SocketPath()) }
