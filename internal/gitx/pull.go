// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var (
	ErrDirtyTree = errors.New("working tree is not clean")
	ErrDiverged  = errors.New("branches have diverged")
	ErrStrategy  = errors.New("pull strategy is not allowed")
)

type PullRequest struct {
	Strategy  string
	Autostash bool
}

type PullResult struct {
	Strategy     string   `json:"strategy"`
	OrigHead     string   `json:"origHead,omitempty"`
	Head         string   `json:"head,omitempty"`
	Conflicts    []string `json:"conflicts,omitempty"`
	StashApplied bool     `json:"stashApplied"`
	Output       string   `json:"output,omitempty"`
}

func (g *Git) Pull(ctx context.Context, repo Repository, request PullRequest, allowed []string) (PullResult, error) {
	strategy := request.Strategy
	if strategy == "" {
		strategy = "ff-only"
	}
	if strategy != "ff-only" && strategy != "merge" && strategy != "rebase" {
		return PullResult{}, ErrStrategy
	}
	if len(allowed) == 0 {
		allowed = []string{"ff-only"}
	}
	ok := false
	for _, value := range allowed {
		if strings.TrimSpace(value) == strategy {
			ok = true
		}
	}
	if !ok {
		return PullResult{}, fmt.Errorf("%w: %s", ErrStrategy, strategy)
	}
	if repo.InProgress != "" {
		return PullResult{}, fmt.Errorf("%w: %s", ErrInProgress, repo.InProgress)
	}
	if repo.Upstream == "" {
		return PullResult{}, errors.New("upstream is not configured")
	}
	if out, err := g.Run(ctx, "status", "--porcelain"); err != nil {
		return PullResult{}, err
	} else if strings.TrimSpace(string(out)) != "" && !request.Autostash {
		return PullResult{}, ErrDirtyTree
	}
	orig, err := g.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return PullResult{}, err
	}
	result := PullResult{Strategy: strategy, OrigHead: strings.TrimSpace(string(orig))}
	if request.Autostash {
		if _, err := g.mutate(ctx, "stash", "push", "--include-untracked", "-m", "runwisp-manager-autostash"); err != nil {
			return PullResult{}, fmt.Errorf("create autostash: %w", err)
		}
	}
	args := []string{"merge", "--ff-only", "@{u}"}
	if strategy == "merge" {
		args = []string{"merge", "--no-edit", "@{u}"}
	}
	if strategy == "rebase" {
		args = []string{"rebase", "@{u}"}
	}
	out, pullErr := g.mutate(ctx, args...)
	if pullErr != nil {
		result.Conflicts = g.conflicts(ctx)
		if strategy == "ff-only" && strings.Contains(strings.ToLower(pullErr.Error()), "fast-forward") {
			return result, ErrDiverged
		}
		return result, pullErr
	}
	if request.Autostash {
		if _, err := g.mutate(ctx, "stash", "pop"); err != nil {
			result.Conflicts = g.conflicts(ctx)
			return result, fmt.Errorf("apply autostash: %w", err)
		}
		result.StashApplied = true
	}
	result.Output = redact(string(out))
	if head, err := g.Run(ctx, "rev-parse", "HEAD"); err == nil {
		result.Head = strings.TrimSpace(string(head))
	}
	return result, nil
}

func (g *Git) conflicts(ctx context.Context) []string {
	out, err := g.Run(ctx, "status", "--porcelain=v2", "-z")
	if err != nil {
		return nil
	}
	var result []string
	for _, record := range strings.Split(string(out), "\x00") {
		if strings.HasPrefix(record, "u ") {
			fields := strings.Fields(record)
			if len(fields) > 0 {
				result = append(result, fields[len(fields)-1])
			}
		}
	}
	return result
}

func (g *Git) Abort(ctx context.Context, operation string) error {
	if operation == "rebase" {
		_, err := g.mutate(ctx, "rebase", "--abort")
		return err
	}
	_, err := g.mutate(ctx, "merge", "--abort")
	return err
}
