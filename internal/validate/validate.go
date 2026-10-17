// SPDX-License-Identifier: GPL-3.0-or-later
// Package validate combines fast schema feedback with authoritative RunWisp validation.
package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/runwisp/runwisp-manager/internal/runwispbin"
	"github.com/runwisp/runwisp-manager/internal/schema"
)

type Issue struct {
	Severity  string `json:"severity"`
	Source    string `json:"source"`
	File      string `json:"file"`
	Line      int    `json:"line,omitempty"`
	Column    int    `json:"column,omitempty"`
	FieldPath string `json:"fieldPath,omitempty"`
	Pointer   string `json:"pointer,omitempty"`
	Message   string `json:"message"`
	UnitKind  string `json:"unitKind,omitempty"`
	UnitName  string `json:"unitName,omitempty"`
}
type Schedule struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}
type Result struct {
	Valid       bool       `json:"valid"`
	Degraded    bool       `json:"degraded,omitempty"`
	Errors      []Issue    `json:"errors,omitempty"`
	Warnings    []Issue    `json:"warnings,omitempty"`
	Schedules   []Schedule `json:"schedules,omitempty"`
	EnvWarnings []string   `json:"envWarnings,omitempty"`
}
type Candidate struct {
	Path    string `json:"path"`
	Content []byte `json:"content"`
}
type Options struct {
	Binary    runwispbin.Runner
	Schema    schema.Document
	Root      string
	StateDir  string
	DaemonPID int
	Mode      string
	Env       []string
}

var slots = make(chan struct{}, 4)

func Quick(data []byte, doc schema.Document, file string) Result {
	var value map[string]any
	if err := toml.Unmarshal(data, &value); err != nil {
		return Result{Valid: false, Errors: []Issue{{Severity: "error", Source: "schema", File: file, Message: fmt.Sprintf("parse TOML: %v", err)}}}
	}
	errs := walk(doc.Resolved, value, "", "")
	for i := range errs {
		errs[i].File = file
		errs[i].Source = "schema"
		errs[i].Severity = "error"
	}
	return Result{Valid: len(errs) == 0, Errors: errs}
}

func Validate(ctx context.Context, opts Options, candidates []Candidate) (Result, error) {
	var result Result
	byPath := map[string][]byte{}
	for _, candidate := range candidates {
		path, err := filepath.Abs(candidate.Path)
		if err != nil {
			return Result{}, fmt.Errorf("resolve candidate: %w", err)
		}
		byPath[path] = candidate.Content
		result = mergeResult(result, Quick(candidate.Content, opts.Schema, path))
	}
	if opts.Mode == "quick" {
		result.Valid = len(result.Errors) == 0
		return result, nil
	}
	select {
	case slots <- struct{}{}:
		defer func() { <-slots }()
	case <-ctx.Done():
		return Result{}, ctx.Err()
	}
	shadow, err := NewShadow(ctx, opts.Root, opts.StateDir, byPath)
	if err != nil {
		return Result{}, err
	}
	defer shadow.Close()
	env, warnings, err := ValidationEnv(opts.Env, opts.DaemonPID)
	if err != nil {
		return Result{}, err
	}
	result.EnvWarnings = warnings
	runner := opts.Binary
	runner.Env = env
	authoritative, err := runner.Validate(ctx, shadow.Root)
	if err != nil {
		result.Degraded = true
		result.Valid = len(result.Errors) == 0
		return result, nil
	}
	result.Valid = authoritative.Valid
	for _, item := range authoritative.Errors {
		result.Errors = append(result.Errors, normalise(item, opts.Root, shadow.Root))
	}
	for _, item := range authoritative.Warnings {
		result.Warnings = append(result.Warnings, normalise(item, opts.Root, shadow.Root))
	}
	result.Errors = dedupe(result.Errors)
	result.Warnings = dedupe(result.Warnings)
	return result, nil
}

func normalise(item runwispbin.ValidationMessage, root, shadowRoot string) Issue {
	file := root
	message := item.Message
	if strings.HasPrefix(message, "included config ") {
		rest := strings.TrimPrefix(message, "included config ")
		if i := strings.Index(rest, ": "); i >= 0 {
			file = strings.TrimSpace(rest[:i])
			message = rest[i+2:]
		}
	}
	file = strings.Replace(file, filepath.Dir(shadowRoot), filepath.Dir(root), 1)
	return Issue{Severity: "error", Source: "runwisp", File: file, Line: item.Line, Column: item.Column, FieldPath: item.Key, Message: message}
}
func mergeResult(a, b Result) Result {
	a.Errors = append(a.Errors, b.Errors...)
	a.Warnings = append(a.Warnings, b.Warnings...)
	return a
}
func dedupe(in []Issue) []Issue {
	seen := map[string]bool{}
	out := in[:0]
	for _, i := range in {
		key := i.File + "\x00" + i.FieldPath + "\x00" + strings.ToLower(strings.TrimSpace(i.Message))
		if !seen[key] {
			seen[key] = true
			out = append(out, i)
		}
	}
	return out
}

func walk(schemaValue any, value any, path, pointer string) []Issue {
	obj, ok := schemaValue.(map[string]any)
	if !ok {
		return nil
	}
	var out []Issue
	if required, ok := obj["required"].([]any); ok {
		if m, ok := value.(map[string]any); ok {
			for _, r := range required {
				name, _ := r.(string)
				if _, exists := m[name]; !exists {
					out = append(out, Issue{FieldPath: join(path, name), Pointer: pointer + "/required", Message: "required field is missing"})
				}
			}
		}
	}
	if enum, ok := obj["enum"].([]any); ok {
		found := false
		for _, v := range enum {
			if fmt.Sprint(v) == fmt.Sprint(value) {
				found = true
			}
		}
		if !found {
			out = append(out, Issue{FieldPath: path, Pointer: pointer + "/enum", Message: "value is not allowed"})
		}
	}
	if props, ok := obj["properties"].(map[string]any); ok {
		if m, ok := value.(map[string]any); ok {
			additional := obj["additionalProperties"]
			for name, v := range m {
				child, exists := props[name]
				if !exists {
					if additional == false {
						out = append(out, Issue{FieldPath: join(path, name), Pointer: pointer + "/additionalProperties", Message: "unknown field"})
					}
					continue
				}
				out = append(out, walk(child, v, join(path, name), pointer+"/properties/"+escape(name))...)
			}
		}
	}
	if typ, ok := obj["type"].(string); ok && !typeMatches(typ, value) {
		out = append(out, Issue{FieldPath: path, Pointer: pointer + "/type", Message: "expected " + typ})
	}
	return out
}
func typeMatches(t string, v any) bool {
	switch t {
	case "object":
		_, ok := v.(map[string]any)
		return ok
	case "array":
		switch v.(type) {
		case []any, []string:
			return true
		}
	case "string":
		_, ok := v.(string)
		return ok
	case "boolean":
		_, ok := v.(bool)
		return ok
	case "integer":
		rv := reflect.ValueOf(v)
		return rv.IsValid() && (rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Int64 || rv.Kind() >= reflect.Uint && rv.Kind() <= reflect.Uint64)
	case "number":
		rv := reflect.ValueOf(v)
		return rv.IsValid() && (rv.Kind() >= reflect.Int && rv.Kind() <= reflect.Float64 || rv.Kind() >= reflect.Uint && rv.Kind() <= reflect.Uint64)
	}
	return true
}
func join(a, b string) string {
	if a == "" {
		return b
	}
	return a + "." + b
}
func escape(s string) string { return strings.ReplaceAll(strings.ReplaceAll(s, "~", "~0"), "/", "~1") }

var includedRE = regexp.MustCompile(`^included config (.+?): (.*)$`)

// ValidationEnv constructs the child-only environment. It deliberately never
// returns environment values in warnings or errors.
func ValidationEnv(base []string, daemonPID int) ([]string, []string, error) {
	if base == nil {
		base = os.Environ()
	}
	values := map[string]string{}
	for _, item := range base {
		if k, v, ok := strings.Cut(item, "="); ok {
			values[k] = v
		}
	}
	if path := os.Getenv("RWM_VALIDATE_ENV_FILE"); path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, fmt.Errorf("read validation environment: %w", err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			if k, v, ok := strings.Cut(strings.TrimSpace(line), "="); ok && k != "" {
				values[k] = v
			}
		}
	}
	mode := strings.ToLower(os.Getenv("RWM_VALIDATE_ENV_FROM_DAEMON"))
	if daemonPID > 0 && (mode == "true" || mode == "auto") {
		data, err := os.ReadFile(fmt.Sprintf("/proc/%d/environ", daemonPID))
		if err == nil {
			for _, item := range strings.Split(string(data), "\x00") {
				if k, v, ok := strings.Cut(item, "="); ok {
					values[k] = v
				}
			}
		} else if mode == "true" {
			return nil, nil, fmt.Errorf("read daemon environment: %w", err)
		}
	}
	out := make([]string, 0, len(values))
	for k, v := range values {
		out = append(out, k+"="+v)
	}
	return out, nil, nil
}
