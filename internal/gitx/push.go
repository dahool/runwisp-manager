// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type PushRequest struct {
	Remote         string
	Branches       []string
	OverrideReason string
}

type PushResult struct {
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	Output string `json:"output,omitempty"`
}

func (g *Git) Push(ctx context.Context, repo Repository, request PushRequest) (PushResult, error) {
	remote := request.Remote
	if remote == "" {
		remote = "origin"
	}
	if err := ValidateRef(remote); err != nil {
		return PushResult{}, err
	}
	branch := repo.Branch
	if branch == "" || repo.Detached {
		return PushResult{}, errors.New("cannot push from detached HEAD")
	}
	if len(request.Branches) > 0 {
		allowed := false
		for _, candidate := range request.Branches {
			if candidate == branch {
				allowed = true
			}
		}
		if !allowed && request.OverrideReason == "" {
			return PushResult{}, fmt.Errorf("push branch %q is not allowed", branch)
		}
	}
	if repo.Upstream != "" {
		status, err := g.Run(ctx, "status", "--porcelain=v2", "--branch")
		if err != nil {
			return PushResult{}, err
		}
		for _, line := range strings.Split(string(status), "\n") {
			if strings.HasPrefix(line, "# branch.ab ") {
				fields := strings.Fields(line)
				if len(fields) == 4 {
					var behind int
					_, _ = fmt.Sscanf(fields[3], "%d", &behind)
					if behind > 0 {
						return PushResult{}, errors.New("repository is behind upstream; pull first")
					}
				}
			}
		}
		out, err := g.Run(ctx, "diff", repo.Upstream+"..HEAD", "--no-color", "--no-ext-diff")
		if err != nil {
			return PushResult{}, err
		}
		if request.OverrideReason == "" && containsSecret(string(out)) {
			return PushResult{}, ErrSecretGuard
		}
	}
	if output, err := g.mutate(ctx, "push", remote, "HEAD:"+branch); err != nil {
		return PushResult{}, err
	} else {
		return PushResult{Remote: remote, Branch: branch, Output: redact(string(output))}, nil
	}
}
