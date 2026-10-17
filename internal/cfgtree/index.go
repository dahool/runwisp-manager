package cfgtree

import (
	"bytes"
	"encoding/json"
	"os"
	"sort"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Index decodes all readable files in tree and fills tree.Units. A malformed
// file remains in Files with its error and does not prevent other files from
// being indexed.
func Index(tree Tree) Tree {
	var units []Unit
	for _, file := range tree.Files {
		if !file.Readable || file.Outside {
			continue
		}
		data, err := os.ReadFile(file.Path)
		if err != nil {
			continue
		}
		data = bytes.TrimPrefix(data, []byte{0xef, 0xbb, 0xbf})
		var document map[string]any
		if err := unmarshalTOML(data, &document); err != nil {
			continue
		}
		lines := tableLines(data)
		walkTables(document, nil, file, lines, &units)
	}
	seen := make(map[string]int)
	for i := range units {
		key := units[i].Kind + "\x00" + units[i].Name
		seen[key]++
	}
	for i := range units {
		key := units[i].Kind + "\x00" + units[i].Name
		units[i].Duplicate = seen[key] > 1
	}
	sort.SliceStable(units, func(i, j int) bool {
		if units[i].File != units[j].File {
			return units[i].File < units[j].File
		}
		return units[i].Line[0] < units[j].Line[0]
	})
	tree.Units = units
	return tree
}

func unmarshalTOML(data []byte, out *map[string]any) error {
	// go-toml's public decoder is kept behind this helper so all input
	// normalization (especially optional UTF-8 BOMs) happens in one place.
	return toml.Unmarshal(data, out)
}

func walkTables(value map[string]any, path []string, file File, lines map[string][2]int, units *[]Unit) {
	for key, child := range value {
		childMap, ok := child.(map[string]any)
		if !ok {
			continue
		}
		current := append(append([]string(nil), path...), key)
		kind, name := classify(current)
		if kind != "" && shouldIndex(current, childMap) {
			fields := cloneMap(childMap)
			encoded, _ := json.Marshal(fields)
			line := lines[strings.Join(current, ".")]
			if line == [2]int{} {
				line = lines[key]
			}
			*units = append(*units, Unit{Kind: kind, Name: name, File: file.Path, Line: line, Hash: Hash(encoded), Fields: fields})
		}
		walkTables(childMap, current, file, lines, units)
	}
}

func shouldIndex(path []string, fields map[string]any) bool {
	if len(path) != 1 {
		return true
	}
	// [task.alpha] is represented by a parent map under generic decoding. The
	// parent is a namespace, not a second task unit.
	for _, value := range fields {
		if _, ok := value.(map[string]any); ok {
			return false
		}
	}
	return true
}

func classify(path []string) (string, string) {
	if len(path) == 0 {
		return "", ""
	}
	known := map[string]string{
		"task": "task", "service": "service", "defaults": "defaults", "daemon": "daemon",
		"storage": "storage", "notify": "notify", "notifier": "notifier", "notifiers": "notifier", "route": "route",
		"notificationRoute": "notificationRoute", "composeAlias": "composeAlias", "composeOverride": "composeOverride",
	}
	key := path[len(path)-1]
	if len(path) == 1 {
		if kind, ok := known[key]; ok {
			return kind, ""
		}
		return "other", key
	}
	if kind, ok := known[path[0]]; ok {
		return kind, strings.Join(path[1:], ".")
	}
	// Nested schema groups such as notifications.routes.name retain a useful
	// display kind while remaining forward-compatible with new schema tables.
	if len(path) >= 2 && (key == "routes" || key == "notifiers") {
		return "other", strings.Join(path[1:], ".")
	}
	return "other", strings.Join(path, ".")
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = cloneValue(value)
	}
	return out
}
func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMap(v)
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = cloneValue(v[i])
		}
		return out
	default:
		return value
	}
}

func tableLines(data []byte) map[string][2]int {
	lines := strings.Split(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n")
	out := make(map[string][2]int)
	for i, raw := range lines {
		text := strings.TrimSpace(raw)
		if !strings.HasPrefix(text, "[") || strings.HasPrefix(text, "[[") {
			continue
		}
		end := strings.IndexByte(text, ']')
		if end < 0 {
			continue
		}
		key := strings.TrimSpace(text[1:end])
		out[key] = [2]int{i + 1, len(lines)}
	}
	keys := make([]string, 0, len(out))
	for key := range out {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return strings.Count(keys[i], ".") > strings.Count(keys[j], ".") })
	for i, key := range keys {
		start := out[key][0]
		end := len(lines)
		for _, other := range keys[:i] {
			if candidate := out[other][0]; candidate > start && candidate < end && strings.HasPrefix(other, key+".") {
				end = candidate - 1
			}
		}
		out[key] = [2]int{start, end}
	}
	return out
}
