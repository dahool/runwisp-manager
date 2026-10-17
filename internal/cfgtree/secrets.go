package cfgtree

import (
	"errors"
	"regexp"
	"strings"
)

var secretKey = regexp.MustCompile(`(?i)(pass(word)?|secret|token|api[_-]?key|webhook|credential|private)`)

// MaskedFields returns a defensive, JSON-friendly copy. Masked values do not
// expose their length; references are intentionally preserved because they do
// not contain the referenced secret.
func MaskedFields(fields map[string]any) map[string]any { return maskMap(fields, nil) }

func maskMap(fields map[string]any, path []string) map[string]any {
	out := make(map[string]any, len(fields))
	for key, value := range fields {
		current := append(append([]string(nil), path...), key)
		switch v := value.(type) {
		case map[string]any:
			out[key] = maskMap(v, current)
		case []any:
			items := make([]any, len(v))
			for i, item := range v {
				items[i] = maskValue(item, current)
			}
			out[key] = items
		default:
			out[key] = maskValueForKey(key, v)
		}
	}
	return out
}

func maskValue(value any, path []string) any {
	if child, ok := value.(map[string]any); ok {
		return maskMap(child, path)
	}
	if text, ok := value.(string); ok && len(path) > 0 {
		return maskValueForKey(path[len(path)-1], text)
	}
	return value
}

func maskValueForKey(key string, value any) any {
	text, ok := value.(string)
	if !ok || !secretKey.MatchString(key) || isReference(text) {
		return value
	}
	return map[string]any{"value": "••••", "masked": true}
}

func isReference(value string) bool {
	value = strings.TrimSpace(value)
	return strings.HasPrefix(value, "${") && strings.HasSuffix(value, "}")
}

// Reveal resolves a dotted key path in a unit. It is deliberately separate
// from HTTP role checks so the API layer can require admin before calling it.
func Reveal(unit Unit, keyPath string) (any, error) {
	parts := strings.Split(keyPath, ".")
	var current any = unit.Fields
	for _, part := range parts {
		fields, ok := current.(map[string]any)
		if !ok {
			return nil, errors.New("key path does not name a field")
		}
		current, ok = fields[part]
		if !ok {
			return nil, errors.New("key path not found")
		}
	}
	return current, nil
}
