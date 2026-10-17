// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var (
	ErrSecretGuard = errors.New("secret guard blocked commit")
	ErrBranchGuard = errors.New("branch guard blocked commit")
	ErrInProgress  = errors.New("git operation is in progress")
	ErrScopeGuard  = errors.New("path is outside the commit scope")
	ErrEmptyCommit = errors.New("nothing staged for commit")
)

func (g *Git) CheckGuards(ctx context.Context, repo Repository, paths []string, options CommitOptions, overrideReason string) error {
	if repo.InProgress != "" {
		return fmt.Errorf("%w: %s", ErrInProgress, repo.InProgress)
	}
	if options.ExpectedBranch != "" && (repo.Detached || repo.Branch != options.ExpectedBranch) && overrideReason == "" {
		return fmt.Errorf("%w: expected %s", ErrBranchGuard, options.ExpectedBranch)
	}
	denied := options.DenyGlobs
	if len(denied) == 0 {
		denied = []string{"**/*.key", "**/*.pem", "**/.env*", "*.key", "*.pem", ".env*"}
	}
	for _, path := range paths {
		if err := ValidatePath(path); err != nil {
			return fmt.Errorf("%w: %s", ErrScopeGuard, path)
		}
		for _, pattern := range denied {
			matched, err := doublestar.Match(pattern, path)
			if err == nil && matched {
				return fmt.Errorf("%w: %s", ErrScopeGuard, path)
			}
		}
	}
	if options.SecretGuard && overrideReason == "" {
		for _, path := range paths {
			literal, err := LiteralPath(path)
			if err != nil {
				return err
			}
			out, runErr := g.Run(ctx, "diff", "HEAD", "--no-color", "--no-ext-diff", "--", literal)
			if runErr != nil {
				// An unborn HEAD or an untracked path has no diff base.
				if data, readErr := os.ReadFile(filepath.Join(g.repo, filepath.FromSlash(path))); readErr == nil {
					out = []byte(addLines(string(data)))
					runErr = nil
				}
			}
			if runErr == nil && containsSecret(string(out)) {
				return fmt.Errorf("%w: %s", ErrSecretGuard, path)
			}
		}
	}
	return nil
}

func containsSecret(diff string) bool {
	for _, line := range strings.Split(diff, "\n") {
		if !strings.HasPrefix(line, "+") || strings.HasPrefix(line, "+++") {
			continue
		}
		value := strings.TrimSpace(line[1:])
		if strings.Contains(value, "${") || strings.Contains(value, "${file:") {
			continue
		}
		if strings.Contains(value, "-----BEGIN ") || privateBlock.MatchString(value) || tokenValue.MatchString(value) || secretAssignment.MatchString(value) {
			return true
		}
	}
	return false
}
