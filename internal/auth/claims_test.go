package auth

import (
	"testing"

	"github.com/runwisp/runwisp-manager/internal/config"
)

func TestNormalizeGroups(t *testing.T) {
	tests := []struct {
		name  string
		value any
		want  []string
	}{
		{name: "array", value: []any{"rwm-viewer", "rwm-admin"}, want: []string{"rwm-viewer", "rwm-admin"}},
		{name: "space and comma string", value: "rwm-viewer, rwm-editor rwm-viewer", want: []string{"rwm-viewer", "rwm-editor"}},
		{name: "string array", value: []string{"one", "two"}, want: []string{"one", "two"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeGroups(tt.value)
			if len(got) != len(tt.want) {
				t.Fatalf("groups = %#v, want %#v", got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("groups = %#v, want %#v", got, tt.want)
				}
			}
		})
	}
}

func TestResolveIdentityRoleMatrix(t *testing.T) {
	tests := []struct {
		name, groups, want string
		defaultRole        string
	}{
		{name: "highest role wins", groups: "viewer,admin", want: "admin"},
		{name: "editor", groups: "editor", want: "editor"},
		{name: "default", groups: "unmapped", want: "viewer", defaultRole: "viewer"},
		{name: "denied", groups: "unmapped"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := config.Config{OIDCClaimUsername: "preferred_username", OIDCClaimName: "name", OIDCClaimEmail: "email", OIDCClaimGroups: "groups", OIDCRoleAdminGroups: "admin", OIDCRoleEditorGroups: "editor", OIDCRoleOperatorGroups: "operator", OIDCRoleViewerGroups: "viewer", OIDCDefaultRole: tt.defaultRole}
			got, err := ResolveIdentity(c, Claims{"sub": "s", "preferred_username": "u", "email": "u@example.com", "groups": tt.groups})
			if tt.want == "" {
				if err == nil {
					t.Fatal("expected denial")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Role != tt.want {
				t.Fatalf("role = %q, want %q", got.Role, tt.want)
			}
		})
	}
}

func TestValidateReturnTo(t *testing.T) {
	tests := []struct {
		name, value string
		wantErr     bool
	}{
		{name: "relative", value: "/tasks/foo"}, {name: "fallback", value: ""}, {name: "scheme", value: "https://evil.example", wantErr: true}, {name: "double slash", value: "//evil.example", wantErr: true}, {name: "backslash", value: `/\\evil`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validateReturnTo(tt.value, "/")
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
