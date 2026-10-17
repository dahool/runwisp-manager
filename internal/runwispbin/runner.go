// SPDX-License-Identifier: GPL-3.0-or-later
// Package runwispbin invokes the RunWisp binary without a shell.
package runwispbin

import (
	"bytes"
	"context"
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

func (r Runner) Run(ctx context.Context, args ...string) (Result, error) {
	timeout := r.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, r.Binary, args...)
	cmd.Env = r.Env
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
