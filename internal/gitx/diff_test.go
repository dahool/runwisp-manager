package gitx

import (
	"context"
	"strings"
	"testing"
)

func TestMaskDiff(t *testing.T) {
	diff := "+password = \"planted-secret\"\n+token = \"ghp_123456789012345678\"\n+value = \"${VAR}\"\n"
	masked := MaskDiff(diff)
	for _, secret := range []string{"planted-secret", "ghp_123456789012345678"} {
		if strings.Contains(masked, secret) {
			t.Fatalf("diff leaked %q: %s", secret, masked)
		}
	}
	if !strings.Contains(masked, "${VAR}") {
		t.Fatal("variable reference should remain visible")
	}
}

func TestLogRejectsInjectionPath(t *testing.T) {
	var g *Git
	if _, err := g.Log(context.Background(), ":(top)config.toml", 1); err == nil {
		t.Fatal("expected pathspec magic to be rejected")
	}
}
