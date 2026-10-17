// SPDX-License-Identifier: GPL-3.0-or-later
// Package schema loads and prepares the RunWisp JSON schema for the manager.
package schema

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type FieldHint struct {
	Pointer     string `json:"pointer"`
	Widget      string `json:"widget,omitempty"`
	Group       string `json:"group,omitempty"`
	Order       int    `json:"order,omitempty"`
	Label       string `json:"label,omitempty"`
	Help        string `json:"help,omitempty"`
	Secret      bool   `json:"secret,omitempty"`
	Advanced    bool   `json:"advanced,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	RefKind     string `json:"refKind,omitempty"`
}

type Overlay struct {
	SchemaHash    string              `json:"schemaHash"`
	Fields        []FieldHint         `json:"fields"`
	NotifierTypes map[string][]string `json:"notifierTypes,omitempty"`
}

type Document struct {
	Raw          map[string]any `json:"schema"`
	Resolved     map[string]any `json:"resolvedSchema"`
	Hash         string         `json:"schemaHash"`
	Overlay      Overlay        `json:"overlay"`
	OverlayDrift bool           `json:"overlayDrift"`
	Version      string         `json:"runwispVersion,omitempty"`
}

// Load validates a schema document, stores it by content hash, and returns a
// resolved view. Cache writes are best effort: validation must still work when
// the state directory is read-only.
func Load(data []byte, stateDir, overlayHash, version string) (Document, error) {
	var raw map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&raw); err != nil {
		return Document{}, fmt.Errorf("decode runwisp schema: %w", err)
	}
	if _, ok := raw["$schema"]; !ok {
		return Document{}, fmt.Errorf("runwisp schema: missing $schema")
	}
	sum := sha256.Sum256(data)
	hash := hex.EncodeToString(sum[:])
	if stateDir != "" {
		if err := cache(stateDir, hash, data); err != nil {
			// The cache is an optimization, not a reason to reject a valid schema.
		}
	}
	overlay := DefaultOverlay(raw, hash)
	return Document{Raw: raw, Resolved: Resolve(raw), Hash: hash, Overlay: overlay, OverlayDrift: overlayHash != "" && overlayHash != hash, Version: version}, nil
}

func cache(stateDir, hash string, data []byte) error {
	dir := filepath.Join(stateDir, "schema")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	path := filepath.Join(dir, hash+".json")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	tmp, err := os.CreateTemp(dir, ".schema-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// DefaultOverlay gives every schema property a generic, safe UI entry. The
// checked-in overlay can later replace individual hints without losing newly
// added RunWisp fields; unknown fields therefore remain visible as Advanced.
func DefaultOverlay(raw map[string]any, hash string) Overlay {
	var fields []FieldHint
	defs, _ := raw["$defs"].(map[string]any)
	for name, value := range defs {
		walkHints(value, "#/$defs/"+escape(name), &fields)
	}
	return Overlay{SchemaHash: hash, Fields: fields, NotifierTypes: map[string][]string{"telegram": {"token", "chatId"}}}
}

func walkHints(value any, pointer string, out *[]FieldHint) {
	obj, ok := value.(map[string]any)
	if !ok {
		return
	}
	if properties, ok := obj["properties"].(map[string]any); ok {
		for name, child := range properties {
			p := pointer + "/properties/" + escape(name)
			widget := "text"
			if childObj, ok := child.(map[string]any); ok {
				switch childObj["type"] {
				case "boolean":
					widget = "bool"
				case "number", "integer":
					widget = "number"
				case "array":
					widget = "list"
				case "object":
					widget = "keyvalue"
				}
				if _, ok := childObj["enum"]; ok {
					widget = "enum"
				}
			}
			*out = append(*out, FieldHint{Pointer: p, Widget: widget, Group: "Advanced", Label: name, Advanced: true})
			walkHints(child, p, out)
		}
	}
}

func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }

// Resolve inlines local $refs and merges allOf objects. Cycles are retained as
// refs, making the result finite and usable by clients.
func Resolve(raw map[string]any) map[string]any {
	return resolveValue(raw, raw, map[string]bool{}).(map[string]any)
}

func resolveValue(value any, root map[string]any, stack map[string]bool) any {
	obj, ok := value.(map[string]any)
	if !ok {
		return value
	}
	if ref, ok := obj["$ref"].(string); ok && strings.HasPrefix(ref, "#/") {
		if stack[ref] {
			return clone(obj)
		}
		if target, ok := pointer(root, strings.TrimPrefix(ref, "#")); ok {
			stack[ref] = true
			resolved := resolveValue(target, root, stack)
			delete(stack, ref)
			merged := map[string]any{}
			for k, v := range resolved.(map[string]any) {
				merged[k] = v
			}
			for k, v := range obj {
				if k != "$ref" {
					merged[k] = resolveValue(v, root, stack)
				}
			}
			return merged
		}
	}
	if all, ok := obj["allOf"].([]any); ok {
		merged := map[string]any{}
		for _, item := range all {
			if itemObj, ok := resolveValue(item, root, stack).(map[string]any); ok {
				merge(merged, itemObj)
			}
		}
		for k, v := range obj {
			if k != "allOf" {
				merged[k] = resolveValue(v, root, stack)
			}
		}
		return merged
	}
	out := map[string]any{}
	for k, v := range obj {
		out[k] = resolveValue(v, root, stack)
	}
	return out
}

func merge(dst, src map[string]any) {
	for k, v := range src {
		if a, ok := dst[k].(map[string]any); ok {
			if b, ok := v.(map[string]any); ok {
				merge(a, b)
				continue
			}
		}
		dst[k] = v
	}
}
func clone(v map[string]any) map[string]any {
	out := map[string]any{}
	for k, x := range v {
		out[k] = x
	}
	return out
}
func pointer(root map[string]any, path string) (any, bool) {
	var cur any = root
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		part = strings.ReplaceAll(strings.ReplaceAll(part, "~1", "/"), "~0", "~")
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		cur, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return cur, true
}
