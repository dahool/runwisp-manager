// SPDX-License-Identifier: GPL-3.0-or-later
// Package runwispbin invokes the RunWisp binary without a shell.
package runwispbin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"
)

const maxOutput = 4 << 20

type Runner struct {
	Binary  string
	Env     []string
	Timeout time.Duration
}
type Result struct {
	Stdout, Stderr []byte
	ExitError      error
}

// ValidationOutput is the machine-readable result emitted by RunWisp.
type ValidationOutput struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Valid         bool                `json:"valid"`
	Errors        []ValidationMessage `json:"errors"`
	Warnings      []ValidationMessage `json:"warnings"`
	Schedules     []json.RawMessage   `json:"schedules"`
}

type ValidationMessage struct {
	Message string `json:"message"`
	Key     string `json:"key"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
}

func (r Runner) Run(ctx context.Context, args ...string) (Result, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	if r.Env != nil {
		cmd.Env = r.Env
	}
	var stdout, stderr cappedBuffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, fmt.Errorf("run %s: %w", r.Binary, ctx.Err())
	}
	return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitError: err}, nil
}

func (r Runner) Version(ctx context.Context) (string, error) {
	result, err := r.Run(ctx, "version")
	if err != nil {
		return "", err
	}
	if result.ExitError != nil {
		return "", fmt.Errorf("runwisp version: %w: %s", result.ExitError, strings.TrimSpace(string(result.Stderr)))
	}
	match := regexp.MustCompile(`(?m)\b(\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?)\b`).FindSubmatch(result.Stdout)
	if len(match) == 0 {
		return "", fmt.Errorf("parse runwisp version output %q", strings.TrimSpace(string(result.Stdout)))
	}
	return string(match[1]), nil
}

// Schema obtains the binary's JSON schema without invoking a shell.
func (r Runner) Schema(ctx context.Context) ([]byte, error) {
	result, err := r.Run(ctx, "schema")
	if err != nil {
		return nil, err
	}
	if result.ExitError != nil {
		return nil, fmt.Errorf("runwisp schema: %w: %s", result.ExitError, strings.TrimSpace(string(result.Stderr)))
	}
	if !json.Valid(result.Stdout) {
		return nil, fmt.Errorf("runwisp schema: invalid JSON output")
	}
	return append([]byte(nil), result.Stdout...), nil
}

// Validate runs the authoritative validator. The caller supplies a root path
// in a shadow tree; the runner never mutates configuration files.
func (r Runner) Validate(ctx context.Context, root string) (ValidationOutput, error) {
	previous := r.Timeout
	r.Timeout = 10 * time.Second
	result, err := r.Run(ctx, "validate", "--json", root)
	r.Timeout = previous
	if err != nil {
		return ValidationOutput{}, err
	}
	if len(result.Stdout) == 0 {
		if result.ExitError != nil {
			return ValidationOutput{}, fmt.Errorf("runwisp validate: %w: %s", result.ExitError, strings.TrimSpace(string(result.Stderr)))
		}
		return ValidationOutput{}, fmt.Errorf("runwisp validate: empty JSON output")
	}
	var output ValidationOutput
	if err := json.Unmarshal(result.Stdout, &output); err != nil {
		return ValidationOutput{}, fmt.Errorf("decode runwisp validation: %w", err)
	}
	if result.ExitError != nil && output.Valid {
		return ValidationOutput{}, fmt.Errorf("runwisp validate: %w", result.ExitError)
	}
	return output, nil
}

type cappedBuffer struct {
	data   bytes.Buffer
	capped bool
}

func (b *cappedBuffer) Write(p []byte) (int, error) {
	remaining := maxOutput - b.data.Len()
	if remaining <= 0 {
		b.capped = true
		return len(p), nil
	}
	if len(p) > remaining {
		_, _ = b.data.Write(p[:remaining])
		b.capped = true
		return len(p), nil
	}
	return b.data.Write(p)
}
func (b *cappedBuffer) Bytes() []byte { return b.data.Bytes() }

type VersionPolicy struct {
	Supported bool
	Warning   string
}

func CheckVersion(version string) VersionPolicy {
	major, minor, _, ok := parse(version)
	if !ok || major != 1 || minor < 7 {
		return VersionPolicy{Warning: fmt.Sprintf("unsupported RunWisp version %q; supported range is >=1.7.0 <2.0.0", version)}
	}
	return VersionPolicy{Supported: true}
}
func parse(version string) (int, int, int, bool) {
	version = strings.TrimPrefix(version, "v")
	parts := strings.SplitN(version, ".", 3)
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	var n [3]int
	for i, part := range parts {
		part = strings.FieldsFunc(part, func(r rune) bool { return r == '-' || r == '+' })[0]
		var err error
		n[i], err = atoi(part)
		if err != nil {
			return 0, 0, 0, false
		}
	}
	return n[0], n[1], n[2], true
}
func atoi(s string) (int, error) {
	var n int
	if s == "" {
		return 0, fmt.Errorf("empty version component")
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid version component")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
