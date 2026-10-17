// SPDX-License-Identifier: GPL-3.0-or-later
// Package gitx provides a narrowly-scoped, safe Git CLI integration.
package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxOutput = 4 << 20

var validRef = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)

// Options controls the Git runner. Repo must be an absolute, existing
// directory. StateDir is manager-owned state and is used for Git's HOME and
// global configuration, keeping user and system Git configuration out.
type Options struct {
	Repo       string
	Bin        string
	StateDir   string
	Timeout    time.Duration
	AllowHooks bool
}

// Git executes Git without a shell and serializes mutating operations.
type Git struct {
	repo       string
	bin        string
	stateDir   string
	timeout    time.Duration
	allowHooks bool
	mutateMu   sync.Mutex
}

// New constructs a runner after validating its repository and state paths.
func New(options Options) (*Git, error) {
	if options.Repo == "" {
		return nil, errors.New("git repository is required")
	}
	repo, err := filepath.Abs(options.Repo)
	if err != nil {
		return nil, fmt.Errorf("resolve git repository: %w", err)
	}
	info, err := os.Stat(repo)
	if err != nil {
		return nil, fmt.Errorf("stat git repository: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("git repository is not a directory")
	}
	state := options.StateDir
	if state == "" {
		state = filepath.Join(repo, ".runwisp-manager")
	}
	state, err = filepath.Abs(state)
	if err != nil {
		return nil, fmt.Errorf("resolve git state directory: %w", err)
	}
	if err := os.MkdirAll(state, 0o700); err != nil {
		return nil, fmt.Errorf("create git state directory: %w", err)
	}
	bin := options.Bin
	if bin == "" {
		bin = "git"
	}
	timeout := options.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &Git{repo: repo, bin: bin, stateDir: state, timeout: timeout, allowHooks: options.AllowHooks}, nil
}

func (g *Git) Repo() string { return g.repo }

// Run is the read-only runner entry point. Callers must provide Git arguments,
// never a shell fragment.
func (g *Git) Run(ctx context.Context, args ...string) ([]byte, error) {
	return g.run(ctx, false, args...)
}

func (g *Git) mutate(ctx context.Context, args ...string) ([]byte, error) {
	return g.mutateInput(ctx, nil, args...)
}

func (g *Git) mutateInput(ctx context.Context, input []byte, args ...string) ([]byte, error) {
	g.mutateMu.Lock()
	defer g.mutateMu.Unlock()
	return g.runInput(ctx, true, input, args...)
}

func (g *Git) run(parent context.Context, mutating bool, args ...string) ([]byte, error) {
	return g.runInput(parent, mutating, nil, args...)
}

func (g *Git) runInput(parent context.Context, mutating bool, input []byte, args ...string) ([]byte, error) {
	if g == nil {
		return nil, errors.New("git runner is nil")
	}
	for _, arg := range args {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return nil, errors.New("git argument contains control characters")
		}
	}
	ctx, cancel := context.WithTimeout(parent, g.timeout)
	defer cancel()
	gitArgs := []string{"-C", g.repo, "-c", "safe.directory=" + g.repo}
	if !g.allowHooks {
		gitArgs = append(gitArgs, "-c", "core.hooksPath=/dev/null")
	}
	gitArgs = append(gitArgs, args...)
	cmd := exec.CommandContext(ctx, g.bin, gitArgs...)
	cmd.Env = g.environment(mutating)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &limitedWriter{dst: &stdout, limit: maxOutput}
	cmd.Stderr = &limitedWriter{dst: &stderr, limit: maxOutput}
	if err := cmd.Run(); err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("git command timed out: %w", ctx.Err())
		}
		message := redact(stderr.String())
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("git command failed: %s", message)
	}
	return stdout.Bytes(), nil
}

func (g *Git) environment(mutating bool) []string {
	pathValue := os.Getenv("PATH")
	if pathValue == "" {
		pathValue = "/usr/bin:/bin"
	}
	env := []string{"PATH=" + pathValue, "HOME=" + g.stateDir, "LANG=C", "GIT_TERMINAL_PROMPT=0", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=" + filepath.Join(g.stateDir, "gitconfig")}
	if !mutating {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	return env
}

type limitedWriter struct {
	dst   io.Writer
	limit int64
	n     int64
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	remaining := w.limit - w.n
	if remaining <= 0 {
		return 0, errors.New("git output exceeds limit")
	}
	if int64(len(p)) > remaining {
		p = p[:remaining]
	}
	n, err := w.dst.Write(p)
	w.n += int64(n)
	if err == nil && w.n >= w.limit {
		return n, errors.New("git output exceeds limit")
	}
	return n, err
}

func redact(value string) string {
	value = regexp.MustCompile(`(?i)(https?://)([^/@\s]+):([^/@\s]+)@`).ReplaceAllString(value, `${1}[redacted]@`)
	value = regexp.MustCompile(`(?i)(token|password|secret|authorization)([=:][^\s]+)`).ReplaceAllString(value, `$1=[redacted]`)
	return strings.TrimSpace(value)
}

// ValidateRef rejects option injection and Git ref syntax outside the manager's
// deliberately small subset.
func ValidateRef(ref string) error {
	if ref == "" || strings.HasPrefix(ref, "-") || !validRef.MatchString(ref) {
		return errors.New("invalid git ref")
	}
	return nil
}

// ValidatePath accepts a managed, repository-relative path and rejects Git
// pathspec magic, traversal, absolute paths, and option-like input.
func ValidatePath(path string) error {
	if path == "" || strings.HasPrefix(path, "-") || strings.HasPrefix(path, ":(") || filepath.IsAbs(path) || strings.ContainsAny(path, "\x00\r\n") {
		return errors.New("invalid managed git path")
	}
	clean := filepath.Clean(filepath.FromSlash(path))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("invalid managed git path")
	}
	return nil
}

func LiteralPath(path string) (string, error) {
	if err := ValidatePath(path); err != nil {
		return "", err
	}
	return ":(literal)" + filepath.ToSlash(filepath.Clean(filepath.FromSlash(path))), nil
}
