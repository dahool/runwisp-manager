package schema

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadResolvesRefsAndCachesByHash(t *testing.T) {
	t.Parallel()
	data := []byte(`{"$schema":"https://json-schema.org/draft/2020-12/schema","$defs":{"item":{"type":"object","properties":{"name":{"type":"string"}}}},"type":"object","properties":{"item":{"$ref":"#/$defs/item"}}}`)
	stateDir := t.TempDir()
	doc, err := Load(data, stateDir, "", "1.7.0")
	if err != nil {
		t.Fatal(err)
	}
	if doc.Hash == "" || doc.Resolved["$defs"] == nil {
		t.Fatalf("incomplete document: %#v", doc)
	}
	entries, err := os.ReadDir(filepath.Join(stateDir, "schema"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache entries = %v, err = %v", entries, err)
	}
	if doc.OverlayDrift {
		t.Fatal("unexpected overlay drift")
	}
}

func TestValidateOverlay(t *testing.T) {
	raw := map[string]any{"$defs": map[string]any{"task": map[string]any{"type": "object"}}}
	missing := ValidateOverlay(raw, Overlay{Fields: []FieldHint{{Pointer: "#/$defs/task"}, {Pointer: "#/$defs/nope"}}})
	if len(missing) != 1 || missing[0] != "#/$defs/nope" {
		t.Fatalf("missing = %#v", missing)
	}
}
