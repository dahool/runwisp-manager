package auth

import (
	"net/http/httptest"
	"testing"

	"github.com/runwisp/runwisp-manager/internal/config"
)

func TestCSRFMatrix(t *testing.T) {
	tests := []struct {
		name, method, origin, referer, contentType string
		want                                       bool
	}{
		{name: "safe get", method: "GET", want: true}, {name: "json origin", method: "POST", origin: "https://manager.example", contentType: "application/json", want: true}, {name: "referer fallback", method: "POST", referer: "https://manager.example/manager", contentType: "application/json", want: true}, {name: "wrong origin", method: "POST", origin: "https://evil.example", contentType: "application/json"}, {name: "form post", method: "POST", origin: "https://manager.example", contentType: "application/x-www-form-urlencoded"}, {name: "missing origin", method: "POST", contentType: "application/json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Authenticator{Config: config.Config{ExternalURL: "https://manager.example"}}
			request := httptest.NewRequest(tt.method, "https://manager.example/manager/auth/logout", nil)
			request.Header.Set("Origin", tt.origin)
			request.Header.Set("Referer", tt.referer)
			request.Header.Set("Content-Type", tt.contentType)
			if got := a.csrfOK(request); got != tt.want {
				t.Fatalf("csrf = %v, want %v", got, tt.want)
			}
		})
	}
}
