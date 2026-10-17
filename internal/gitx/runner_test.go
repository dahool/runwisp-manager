package gitx

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidatePath(t *testing.T) {
	tests := []struct {
		name string
		path string
		ok   bool
	}{
		{name: "nested", path: "config/tasks.toml", ok: true},
		{name: "option", path: "--upload-pack=x", ok: false},
		{name: "magic", path: ":(top)config.toml", ok: false},
		{name: "traversal", path: "../secret", ok: false},
		{name: "newline", path: "x\ny", ok: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err == nil) != tt.ok {
				t.Fatalf("ValidatePath(%q) error = %v, want valid=%v", tt.path, err, tt.ok)
			}
		})
	}
}

func TestRunnerSanitizesEnvironmentAndDisablesHooks(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	g, err := New(Options{Repo: root, StateDir: state})
	if err != nil {
		t.Fatal(err)
	}
	if got := g.environment(false); strings.Contains(strings.Join(got, "\n"), "GIT_ASKPASS") {
		t.Fatal("askpass must not be present before phase 12")
	}
	if _, err := g.Run(context.Background(), "--version"); err != nil {
		t.Fatalf("run git: %v", err)
	}
	if _, err := g.Run(context.Background(), "rev-parse", "--is-inside-work-tree"); err == nil {
		t.Fatal("expected empty repository to fail rev-parse")
	}
}

func TestRedact(t *testing.T) {
	got := redact("fatal: https://user:password@example.test/x token=abc123")
	if strings.Contains(got, "password@example") || strings.Contains(got, "abc123") {
		t.Fatalf("redaction leaked secret: %q", got)
	}
}
