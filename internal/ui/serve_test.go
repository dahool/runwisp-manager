package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandlerFallbackAndHeaders(t *testing.T) {
	jsAsset := "/_app/immutable/entry/start.CQDYvKX1.js"
	if matches, err := assets.ReadDir("dist/_app/immutable/entry"); err == nil {
		for _, m := range matches {
			if strings.HasSuffix(m.Name(), ".js") {
				jsAsset = "/_app/immutable/entry/" + m.Name()
				break
			}
		}
	}
	tests := []struct {
		name, path, accept, contentType, cache string
		status                                 int
	}{
		{"root", "/", "text/html", "text/html", "no-cache", http.StatusOK},
		{"deep route", "/tasks/example", "text/html", "text/html", "no-cache", http.StatusOK},
		{"dotted route", "/tasks/a.b", "text/html", "", "", http.StatusNotFound},
		{"asset", "/_app/immutable/chunk.js", "*/*", "", "", http.StatusNotFound},
		{"javascript asset", jsAsset, "*/*", "text/javascript", "public, max-age=31536000, immutable", http.StatusOK},
		{"stylesheet asset", "/_app/immutable/assets/0.DTa4qxZl.css", "*/*", "text/css", "public, max-age=31536000, immutable", http.StatusOK},
		{"api", "/api/nope", "text/html", "application/problem+json", "", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, nil)
			r.Header.Set("Accept", tt.accept)
			w := httptest.NewRecorder()
			Handler().ServeHTTP(w, r)
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			if tt.contentType != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tt.contentType) {
				t.Fatalf("content type = %q", w.Header().Get("Content-Type"))
			}
			if tt.cache != "" && w.Header().Get("Cache-Control") != tt.cache {
				t.Fatalf("cache = %q, want %q", w.Header().Get("Cache-Control"), tt.cache)
			}
		})
	}
}
