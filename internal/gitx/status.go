// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Repository struct {
	Enabled    bool
	Root       string
	Branch     string
	Upstream   string
	Detached   bool
	InProgress string
}

func Detect(ctx context.Context, configDir, override string, options Options) (Repository, *Git, error) {
	root := override
	if root == "" {
		root = configDir
	}
	if root == "" {
		return Repository{}, nil, errors.New("git detection directory is required")
	}
	if override == "" {
		candidate, err := filepath.Abs(root)
		if err != nil {
			return Repository{}, nil, fmt.Errorf("resolve git detection directory: %w", err)
		}
		probe, err := New(Options{Repo: candidate, Bin: options.Bin, StateDir: options.StateDir, Timeout: options.Timeout, AllowHooks: options.AllowHooks})
		if err != nil {
			return Repository{}, nil, nil
		}
		if _, err := probe.Run(ctx, "rev-parse", "--show-toplevel"); err != nil {
			return Repository{}, nil, nil
		}
	}
	g, err := New(Options{Repo: root, Bin: options.Bin, StateDir: options.StateDir, Timeout: options.Timeout, AllowHooks: options.AllowHooks})
	if err != nil {
		return Repository{}, nil, err
	}
	out, err := g.Run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, nil, fmt.Errorf("detect git repository: %w", err)
	}
	repoRoot := filepath.Clean(strings.TrimSpace(string(out)))
	branchOut, branchErr := g.Run(ctx, "symbolic-ref", "--quiet", "--short", "HEAD")
	detached := branchErr != nil
	branch := strings.TrimSpace(string(branchOut))
	upstreamOut, _ := g.Run(ctx, "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}")
	repo := Repository{Enabled: true, Root: repoRoot, Branch: branch, Upstream: strings.TrimSpace(string(upstreamOut)), Detached: detached, InProgress: operationState(repoRoot)}
	return repo, g, nil
}

func operationState(root string) string {
	gitDir := filepath.Join(root, ".git")
	if data, err := os.ReadFile(filepath.Join(gitDir, "MERGE_HEAD")); err == nil && len(data) > 0 {
		return "merge"
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-merge")); err == nil {
		return "rebase"
	}
	if _, err := os.Stat(filepath.Join(gitDir, "rebase-apply")); err == nil {
		return "rebase"
	}
	if _, err := os.Stat(filepath.Join(gitDir, "CHERRY_PICK_HEAD")); err == nil {
		return "cherry-pick"
	}
	return ""
}

type ManagedPath struct {
	ID   string
	Path string
}

type FileStatus struct {
	Path      string `json:"path"`
	FileID    string `json:"fileId,omitempty"`
	Staged    bool   `json:"staged"`
	Unstaged  bool   `json:"unstaged"`
	Untracked bool   `json:"untracked"`
	Unmerged  bool   `json:"unmerged"`
	Renamed   bool   `json:"renamed"`
}

type Status struct {
	Branch       string       `json:"branch"`
	Upstream     string       `json:"upstream,omitempty"`
	Ahead        int          `json:"ahead"`
	Behind       int          `json:"behind"`
	Detached     bool         `json:"detached"`
	InProgress   string       `json:"inProgress,omitempty"`
	Files        []FileStatus `json:"files,omitempty"`
	OtherChanges int          `json:"otherChanges,omitempty"`
	Clean        bool         `json:"clean"`
}

func (g *Git) Status(ctx context.Context, repo Repository, managed []ManagedPath) (Status, error) {
	// Read the complete status so unrelated changes can be counted, then filter
	// managed files in memory. Passing a managed pathspec would hide that count.
	args := []string{"status", "--porcelain=v2", "--branch", "-z"}
	managedByPath := make(map[string]ManagedPath, len(managed))
	for _, file := range managed {
		rel, err := filepath.Rel(g.repo, file.Path)
		if err != nil {
			continue
		}
		if err := ValidatePath(rel); err != nil {
			continue
		}
		managedByPath[filepath.ToSlash(rel)] = file
	}
	out, err := g.Run(ctx, args...)
	if err != nil {
		return Status{}, fmt.Errorf("read git status: %w", err)
	}
	result := Status{Branch: repo.Branch, Upstream: repo.Upstream, Detached: repo.Detached, InProgress: repo.InProgress}
	for _, record := range strings.Split(string(out), "\x00") {
		if record == "" {
			continue
		}
		if strings.HasPrefix(record, "# branch.ab ") {
			fields := strings.Fields(record)
			if len(fields) == 4 {
				result.Ahead, _ = strconv.Atoi(fields[2])
				result.Behind, _ = strconv.Atoi(fields[3])
			}
			continue
		}
		if strings.HasPrefix(record, "# ") {
			continue
		}
		status, path := parseStatusRecord(record)
		if status == nil {
			continue
		}
		if file, ok := managedByPath[filepath.ToSlash(path)]; ok {
			status.FileID = file.ID
			result.Files = append(result.Files, *status)
		} else {
			result.OtherChanges++
		}
	}
	result.Clean = len(result.Files) == 0 && result.OtherChanges == 0 && result.InProgress == ""
	return result, nil
}

func parseStatusRecord(record string) (*FileStatus, string) {
	fields := strings.Fields(record)
	if len(fields) < 2 {
		return nil, ""
	}
	if fields[0] == "?" {
		path := strings.TrimSpace(strings.TrimPrefix(record, "?"))
		return &FileStatus{Path: path, Untracked: true, Unstaged: true}, path
	}
	if fields[0] == "u" && len(fields) >= 11 {
		path := fields[len(fields)-1]
		return &FileStatus{Path: path, Staged: true, Unstaged: true, Unmerged: true}, path
	}
	if fields[0] == "1" && len(fields) >= 9 {
		xy := fields[1]
		path := fields[len(fields)-1]
		return &FileStatus{Path: path, Staged: xy[0] != '.', Unstaged: xy[1] != '.'}, path
	}
	if fields[0] == "2" && len(fields) >= 10 {
		xy := fields[1]
		path := fields[len(fields)-1]
		return &FileStatus{Path: path, Staged: xy[0] != '.', Unstaged: xy[1] != '.', Renamed: true}, path
	}
	return nil, ""
}
