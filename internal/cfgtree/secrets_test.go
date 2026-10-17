package cfgtree

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMaskedFieldsNeverSerializeLiteralSecrets(t *testing.T) {
	t.Parallel()
	fields := map[string]any{
		"password": "planted-password", "api_key": "planted-api-key", "privateKey": "planted-private",
		"endpoint": "${API_TOKEN}", "nested": map[string]any{"webhook": "planted-webhook"},
		"items": []any{map[string]any{"token": "planted-token"}},
	}
	data, err := json.Marshal(MaskedFields(fields))
	if err != nil {
		t.Fatal(err)
	}
	serialized := string(data)
	for _, secret := range []string{"planted-password", "planted-api-key", "planted-private", "planted-webhook", "planted-token"} {
		if strings.Contains(serialized, secret) {
			t.Fatalf("serialized masked fields leaked %q: %s", secret, serialized)
		}
	}
	if !strings.Contains(serialized, "${API_TOKEN}") || strings.Contains(serialized, "length") {
		t.Fatalf("reference/metadata mismatch: %s", serialized)
	}
}

func TestRevealUsesExactKeyPath(t *testing.T) {
	t.Parallel()
	unit := Unit{Fields: map[string]any{"auth": map[string]any{"token": "secret"}}}
	value, err := Reveal(unit, "auth.token")
	if err != nil || value != "secret" {
		t.Fatalf("reveal = %#v, %v", value, err)
	}
	if _, err := Reveal(unit, "auth.missing"); err == nil {
		t.Fatal("missing key unexpectedly revealed")
	}
}
