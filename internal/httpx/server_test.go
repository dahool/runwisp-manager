package httpx

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRouterBasics(t *testing.T) {
	log := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))
	h := NewRouter("1.2.3", "abc", log)

	tests := []struct{ name, path, wantType string; status int }{
		{"health", "/manager/healthz", "application/json", http.StatusOK},
		{"api", "/manager/api/nope", "application/problem+json", http.StatusNotFound},
		{"shell", "/manager/", "text/html", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path+"?token=secret", nil)
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tt.status { t.Fatalf("status = %d, want %d", w.Code, tt.status) }
			if !strings.HasPrefix(w.Header().Get("Content-Type"), tt.wantType) { t.Fatalf("content type = %q", w.Header().Get("Content-Type")) }
			for _, header := range []string{"X-Content-Type-Options", "X-Frame-Options", "Referrer-Policy", "Permissions-Policy", "Content-Security-Policy", "X-Request-ID"} {
				if w.Header().Get(header) == "" { t.Errorf("missing %s", header) }
			}
		})
	}
}

func TestSecurityHeadersHSTSOnlyForHTTPS(t *testing.T) {
	log := slog.New(slog.NewTextHandler(httptest.NewRecorder(), nil))
	h := NewRouter("dev", "dev", log)
	for _, forwarded := range []string{"", "http", "https"} {
		r := httptest.NewRequest(http.MethodGet, "/manager/healthz", nil)
		r.Header.Set("X-Forwarded-Proto", forwarded)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		got := w.Header().Get("Strict-Transport-Security") != ""
		if got != (forwarded == "https") { t.Errorf("forwarded proto %q: HSTS = %v", forwarded, got) }
	}
}
