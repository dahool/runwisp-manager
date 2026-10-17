package ui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/runwisp/runwisp-manager/internal/config"
)

func TestBridgeHandler(t *testing.T) {
	tests := []struct {
		name, position, want, notWant string
		enabled                       bool
		status                        int
	}{
		{name: "enabled bottom right", enabled: true, position: "bottom-right", want: "right:16px;bottom:16px", status: http.StatusOK},
		{name: "enabled top left", enabled: true, position: "top-left", want: "left:16px;top:16px", status: http.StatusOK},
		{name: "disabled", enabled: false, position: "bottom-right", notWant: "RunWisp Manager", status: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			BridgeHandler(config.Config{Bridge: tt.enabled, BridgePosition: tt.position})(w, httptest.NewRequest(http.MethodGet, "/manager/bridge.js", nil))
			if w.Code != tt.status {
				t.Fatalf("status = %d, want %d", w.Code, tt.status)
			}
			if tt.want != "" && !strings.Contains(w.Body.String(), tt.want) {
				t.Fatalf("bridge missing %q", tt.want)
			}
			if tt.notWant != "" && strings.Contains(w.Body.String(), tt.notWant) {
				t.Fatalf("disabled bridge contained %q", tt.notWant)
			}
		})
	}
}
