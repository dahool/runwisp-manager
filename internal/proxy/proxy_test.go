package proxy

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/config"
	"github.com/runwisp/runwisp-manager/internal/daemon"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name, method, path string
		want               Class
	}{
		{"public health", "GET", "/health", Public},
		{"public hook", "POST", "/api/hooks/example", Public},
		{"identity denied", "GET", "/api/daemon/identity", Denied},
		{"auth status", "GET", "/api/auth/status", Synth},
		{"auth mutation denied", "POST", "/api/auth/login", Denied},
		{"read", "GET", "/api/tasks", Read},
		{"operate", "POST", "/api/tasks/demo/run", Operate},
		{"apply", "POST", "/api/daemon/reload", Apply},
		{"unknown mutation", "POST", "/api/new-future-operation", Denied},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classify(tt.method, tt.path, []string{"/health", "/api/hooks/"}, false); got != tt.want {
				t.Fatalf("classify() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHandlerSanitizesBrowserCredentials(t *testing.T) {
	seen := make(chan http.Header, 1)
	fake := newFakeClient("http://runwisp.invalid")
	fake.rt = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		seen <- r.Header.Clone()
		return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/plain"}}, Body: io.NopCloser(strings.NewReader("ok")), Request: r}, nil
	})
	a := &auth.Authenticator{Disabled: true}
	h := New(fake, a, config.Config{ProxyPublicPaths: "/health"}, slog.Default())
	r := httptest.NewRequest(http.MethodGet, "/api/tasks", nil)
	r.RemoteAddr = "192.0.2.20:1234"
	r.Header.Set("Authorization", "Bearer browser")
	r.Header.Set("Cookie", "runwisp_jwt=browser")
	r.Header.Set("X-Forwarded-For", "198.51.100.4")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	got := <-seen
	for _, name := range []string{"Authorization", "Cookie", "Forwarded", "X-Real-IP"} {
		if got.Get(name) != "" {
			t.Errorf("upstream received %s = %q", name, got.Get(name))
		}
	}
	if got.Get("X-Forwarded-For") != "192.0.2.20" || got.Get("X-Forwarded-Proto") != "http" || got.Get("X-Forwarded-Host") != "example.com" {
		t.Errorf("trusted forwarding headers = %q, %q, %q", got.Get("X-Forwarded-For"), got.Get("X-Forwarded-Proto"), got.Get("X-Forwarded-Host"))
	}
}

func TestInjectHTMLGzipAndFallback(t *testing.T) {
	tests := []struct{ name, body string }{{"head", `<html><head></head><body>x</body></html>`}, {"body fallback", `<html><body>x</body></html>`}, {"no marker", `<html>x</html>`}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var compressed strings.Builder
			zw := gzip.NewWriter(&compressed)
			_, _ = zw.Write([]byte(tt.body))
			_ = zw.Close()
			resp := &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(compressed.String()))}
			resp.Header.Set("Content-Type", "text/html")
			resp.Header.Set("Content-Encoding", "gzip")
			if err := injectHTML(resp); err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(data), `/manager/bridge.js`) {
				t.Fatalf("injected script missing: %s", data)
			}
			if resp.Header.Get("Content-Encoding") != "" || resp.Header.Get("Content-Length") == "" {
				t.Fatalf("response encoding/length not normalized")
			}
		})
	}
}

type fakeClient struct {
	base *url.URL
	rt   http.RoundTripper
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func newFakeClient(raw string) *fakeClient {
	u, _ := url.Parse(raw)
	return &fakeClient{base: u, rt: http.DefaultTransport}
}
func (f *fakeClient) Transport() daemon.Transport     { return daemon.TransportHTTP }
func (f *fakeClient) RoundTripper() http.RoundTripper { return f.rt }
func (f *fakeClient) BaseURL() *url.URL               { return f.base }
func (f *fakeClient) Info(context.Context) (daemon.DaemonInfo, error) {
	return daemon.DaemonInfo{}, nil
}
func (f *fakeClient) Identity(context.Context) (daemon.Identity, error) {
	return daemon.Identity{}, nil
}
func (f *fakeClient) Tasks(context.Context) ([]daemon.Task, error) { return nil, nil }
func (f *fakeClient) Reload(context.Context) (daemon.ReloadResult, error) {
	return daemon.ReloadResult{}, nil
}
func (f *fakeClient) Do(ctx context.Context, method, path string, body, out any) error {
	req, _ := http.NewRequestWithContext(ctx, method, f.base.ResolveReference(&url.URL{Path: path}).String(), nil)
	resp, err := f.rt.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return jsonDecode(resp.Body, out)
}
func jsonDecode(body io.Reader, out any) error { return json.NewDecoder(body).Decode(out) }
