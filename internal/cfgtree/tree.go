// SPDX-License-Identifier: GPL-3.0-or-later
// Package cfgtree discovers and indexes the read-only RunWisp configuration.
package cfgtree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/pelletier/go-toml/v2"
)

const (
	maxDepth = 16
	maxFiles = 500
)

// Unit is a decoded configuration table. Fields are safe for presentation
// only; callers must use MaskedFields before returning them to viewers.
type Unit struct {
	Kind      string         `json:"kind"`
	Name      string         `json:"name,omitempty"`
	File      string         `json:"file"`
	Line      [2]int         `json:"line"`
	Hash      string         `json:"hash"`
	Fields    map[string]any `json:"fields,omitempty"`
	Duplicate bool           `json:"duplicate,omitempty"`
}

// File describes one discovered path. Outside files are deliberately metadata
// only and are never read by the indexer.
type File struct {
	ID       string   `json:"id"`
	Path     string   `json:"path"`
	Version  string   `json:"version,omitempty"`
	Size     int64    `json:"size"`
	MTime    int64    `json:"mtime"`
	Root     bool     `json:"root,omitempty"`
	Outside  bool     `json:"outside,omitempty"`
	Readable bool     `json:"readable"`
	Includes []string `json:"includes,omitempty"`
	Error    string   `json:"error,omitempty"`
}

// Tree is the complete discovery result and is safe to marshal as an API
// response after its units have been masked.
type Tree struct {
	Root           string   `json:"root"`
	Files          []File   `json:"files"`
	Warnings       []string `json:"warnings,omitempty"`
	TreeVersion    string   `json:"treeVersion"`
	Units          []Unit   `json:"units,omitempty"`
	Stale          bool     `json:"stale"`
	ReadOnly       bool     `json:"readOnly"`
	ReadOnlyReason string   `json:"readOnlyReason,omitempty"`
}

// IncludePreview resolves include globs against the managed configuration
// root without reading file contents. Invalid or escaping patterns are
// reported as warnings and never produce matches outside the root.
type IncludePreview struct {
	Matches  []string `json:"matches"`
	Warnings []string `json:"warnings,omitempty"`
}

func PreviewIncludes(root string, globs []string) (IncludePreview, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return IncludePreview{}, fmt.Errorf("resolve include root: %w", err)
	}
	result := IncludePreview{}
	seen := make(map[string]bool)
	for _, pattern := range globs {
		if pattern == "" || filepath.IsAbs(pattern) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("include %q is not a relative glob", pattern))
			continue
		}
		candidate := filepath.Join(root, filepath.FromSlash(pattern))
		if !within(root, candidate) {
			result.Warnings = append(result.Warnings, fmt.Sprintf("include %q is outside managed area", pattern))
			continue
		}
		matches, globErr := doublestar.FilepathGlob(candidate)
		if globErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("include %q: %v", pattern, globErr))
			continue
		}
		if len(matches) == 0 {
			result.Warnings = append(result.Warnings, fmt.Sprintf("include %q matched no files", pattern))
		}
		for _, match := range matches {
			match, _ = filepath.Abs(match)
			if within(root, match) && !seen[match] {
				seen[match] = true
				result.Matches = append(result.Matches, match)
			}
		}
	}
	sort.Strings(result.Matches)
	return result, nil
}

// Options controls discovery. Root must be an absolute or resolvable TOML
// file; callers may provide the daemon capability-derived read-only state.
type Options struct {
	Root           string
	ReadOnly       bool
	ReadOnlyReason string
}

// Discover builds an ordered graph starting with the root file. Include
// patterns are resolved relative to the including file, matching RunWisp's
// configuration semantics.
func Discover(opts Options) (Tree, error) {
	root, err := filepath.Abs(opts.Root)
	if err != nil {
		return Tree{}, fmt.Errorf("resolve config root: %w", err)
	}
	root = filepath.Clean(root)
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return Tree{}, fmt.Errorf("resolve config root %q: %w", root, err)
	}
	rootReal, err = filepath.Abs(rootReal)
	if err != nil {
		return Tree{}, err
	}
	allowedRoot := filepath.Dir(rootReal)
	t := Tree{Root: rootReal, ReadOnly: opts.ReadOnly, ReadOnlyReason: opts.ReadOnlyReason}
	seen := make(map[string]bool)
	stack := make(map[string]bool)
	var visit func(string, int, bool)
	visit = func(path string, depth int, isRoot bool) {
		if len(t.Files) >= maxFiles {
			t.Warnings = append(t.Warnings, "include graph exceeds 500 files; remaining files omitted")
			return
		}
		canonical, canonicalErr := filepath.EvalSymlinks(path)
		if canonicalErr != nil {
			t.Warnings = append(t.Warnings, fmt.Sprintf("include %q cannot be resolved: %v", path, canonicalErr))
			return
		}
		canonical, _ = filepath.Abs(canonical)
		outside := !within(allowedRoot, canonical)
		if stack[canonical] {
			t.Warnings = append(t.Warnings, fmt.Sprintf("include cycle detected at %q", canonical))
			return
		}
		if seen[canonical] {
			return
		}
		seen[canonical], stack[canonical] = true, true
		defer delete(stack, canonical)
		f := File{ID: ID(canonical), Path: canonical, Root: isRoot, Outside: outside}
		if outside {
			f.Error = "outside managed area"
			t.Files = append(t.Files, f)
			t.Warnings = append(t.Warnings, fmt.Sprintf("%q is outside managed area", canonical))
			return
		}
		data, readErr := os.ReadFile(canonical)
		if readErr != nil {
			f.Error = readErr.Error()
			t.Files = append(t.Files, f)
			return
		}
		info, statErr := os.Stat(canonical)
		if statErr == nil {
			f.Size, f.MTime, f.Readable = info.Size(), info.ModTime().UnixNano(), true
			f.Version = Hash(data)
		}
		t.Files = append(t.Files, f)
		var raw map[string]any
		if err := toml.Unmarshal(data, &raw); err != nil {
			t.Files[len(t.Files)-1].Error = fmt.Sprintf("parse TOML: %v", err)
			return
		}
		patterns := includePatterns(raw)
		for _, pattern := range patterns {
			base := filepath.Join(filepath.Dir(canonical), pattern)
			matches, globErr := doublestar.FilepathGlob(base)
			if globErr != nil {
				t.Warnings = append(t.Warnings, fmt.Sprintf("include %q: %v", pattern, globErr))
				continue
			}
			if len(matches) == 0 {
				t.Warnings = append(t.Warnings, fmt.Sprintf("include %q matched no files", pattern))
			}
			sort.Strings(matches)
			for _, match := range matches {
				if depth >= maxDepth {
					t.Warnings = append(t.Warnings, fmt.Sprintf("include depth exceeds %d at %q", maxDepth, match))
					break
				}
				if idx := len(t.Files) - 1; idx >= 0 {
					t.Files[idx].Includes = append(t.Files[idx].Includes, ID(match))
				}
				visit(match, depth+1, false)
			}
		}
	}
	visit(root, 0, true)
	parts := make([]string, 0, len(t.Files))
	for _, f := range t.Files {
		parts = append(parts, f.Path+":"+f.Version)
	}
	sort.Strings(parts)
	t.TreeVersion = Hash([]byte(strings.Join(parts, "\n")))
	return t, nil
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}

func includePatterns(raw map[string]any) []string {
	daemon, _ := raw["daemon"].(map[string]any)
	var out []string
	for key, value := range daemon {
		if key != "include" && !strings.HasPrefix(key, "include") {
			continue
		}
		switch v := value.(type) {
		case string:
			out = append(out, v)
		case []string:
			out = append(out, v...)
		case []any:
			for _, item := range v {
				if s, ok := item.(string); ok {
					out = append(out, s)
				}
			}
		}
	}
	sort.Strings(out)
	return out
}

func Hash(data []byte) string { sum := sha256.Sum256(data); return hex.EncodeToString(sum[:]) }

func ID(path string) string { return Hash([]byte(path))[:24] }
