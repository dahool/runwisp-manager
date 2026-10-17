package schema

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed overlay.json
var embeddedOverlay []byte

// EmbeddedOverlay returns the versioned authoring overlay. Runtime-generated
// generic entries are added by DefaultOverlay for schema properties not yet
// explicitly documented here.
func EmbeddedOverlay() (Overlay, error) {
	var value struct {
		Fields        []FieldHint         `json:"fields"`
		NotifierTypes map[string][]string `json:"notifierTypes"`
	}
	if err := json.Unmarshal(embeddedOverlay, &value); err != nil {
		return Overlay{}, err
	}
	return Overlay{Fields: value.Fields, NotifierTypes: value.NotifierTypes}, nil
}

// ValidateOverlay reports overlay pointers that do not exist in a schema.
func ValidateOverlay(raw map[string]any, overlay Overlay) []string {
	var missing []string
	for _, hint := range overlay.Fields {
		if _, ok := pointer(raw, strings.TrimPrefix(hint.Pointer, "#")); !ok {
			missing = append(missing, hint.Pointer)
		}
	}
	return missing
}
