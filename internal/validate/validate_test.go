package validate

import (
	"testing"

	"github.com/runwisp/runwisp-manager/internal/schema"
)

func testDocument() schema.Document {
	return schema.Document{Resolved: map[string]any{"type": "object", "properties": map[string]any{"name": map[string]any{"type": "string"}, "enabled": map[string]any{"type": "boolean"}}, "required": []any{"name"}, "additionalProperties": false}}
}

func TestQuickReturnsAllFieldErrors(t *testing.T) {
	t.Parallel()
	result := Quick([]byte("name = 4\nenabled = \"yes\"\nextra = true\n"), testDocument(), "/tmp/config.toml")
	if len(result.Errors) != 3 {
		t.Fatalf("errors = %#v", result.Errors)
	}
	for _, issue := range result.Errors {
		if issue.Source != "schema" || issue.File != "/tmp/config.toml" {
			t.Fatalf("issue attribution = %#v", issue)
		}
	}
}

func TestValidationEnvDoesNotLeakValues(t *testing.T) {
	t.Setenv("RWM_VALIDATE_ENV_FROM_DAEMON", "false")
	env, warnings, err := ValidationEnv([]string{"SECRET_VALUE=hidden"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 0 || len(env) != 1 || env[0] != "SECRET_VALUE=hidden" {
		t.Fatalf("env = %#v warnings = %#v", env, warnings)
	}
}
