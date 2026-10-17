package cfgsave

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// CheckConflicts compares each requested base version before any computation
// or write. It returns all mismatches so an editor can refresh in one roundtrip.
func CheckConflicts(change Change, files map[FileID][]byte) []Conflict {
	var conflicts []Conflict
	for id, baseHash := range change.Base {
		current, ok := files[id]
		if !ok {
			conflicts = append(conflicts, Conflict{File: id, CurrentHash: "", UnifiedDiff: "file is no longer available"})
			continue
		}
		if hash(current) == baseHash {
			continue
		}
		conflicts = append(conflicts, Conflict{File: id, CurrentHash: hash(current), UnifiedDiff: unifiedDiff(nil, current, string(id))})
	}
	return conflicts
}

// UnifiedDiff is deliberately small and deterministic; it is suitable for a
// conflict preview and does not claim to be a full Myers diff.
func UnifiedDiff(before, after []byte, name string) string { return unifiedDiff(before, after, name) }

func unifiedDiff(before, after []byte, name string) string {
	if bytes.Equal(before, after) {
		return ""
	}
	left := strings.Split(strings.ReplaceAll(string(before), "\r\n", "\n"), "\n")
	right := strings.Split(strings.ReplaceAll(string(after), "\r\n", "\n"), "\n")
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (base)\n+++ %s (current)\n", name, name)
	fmt.Fprintf(&b, "@@ -1,%d +1,%d @@\n", len(left), len(right))
	for _, line := range left {
		b.WriteString("-")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	for _, line := range right {
		b.WriteString("+")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}
