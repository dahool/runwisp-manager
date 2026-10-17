package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoad(t *testing.T) {
	secret := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(secret, []byte("secret\n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name    string
		env     []string
		wantErr string
	}{
		{name: "defaults", env: nil},
		{name: "file secret", env: []string{"RWM_RUNWISP_PASSWORD_FILE=" + secret}},
		{name: "unknown variable", env: []string{"RWM_LISEN=1"}, wantErr: "unknown variable RWM_LISEN"},
		{name: "cross field", env: []string{"RWM_RUNWISP_SOCKET=/tmp/a", "RWM_RUNWISP_URL=http://x", "RWM_TLS_CERT_FILE=x"}, wantErr: "mutually exclusive"},
		{name: "all validation errors", env: []string{"RWM_RUNWISP_SOCKET=/tmp/a", "RWM_RUNWISP_URL=http://x", "RWM_TLS_CERT_FILE=x", "RWM_TRUSTED_PROXIES=0.0.0.0/0", "RWM_EXTERNAL_URL=http://example.com"}, wantErr: "RWM_EXTERNAL_URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, err := Load(tt.env)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatal(err)
				}
				if c.Listen != "127.0.0.1:9478" {
					t.Fatalf("default listen = %q", c.Listen)
				}
				if tt.name == "file secret" && c.RunWispPassword != "secret" {
					t.Fatalf("password = %q", c.RunWispPassword)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestEffectiveRedactsSecrets(t *testing.T) {
	c := Config{RunWispPassword: "password"}
	if got := c.Effective()["RWM_RUNWISP_PASSWORD"]; got != "***" {
		t.Fatalf("secret = %q", got)
	}
}
