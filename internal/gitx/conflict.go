// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ConflictFile struct {
	Path   string `json:"path"`
	Ours   string `json:"ours,omitempty"`
	Theirs string `json:"theirs,omitempty"`
	Both   string `json:"both,omitempty"`
	Binary bool   `json:"binary"`
}

func (g *Git) Conflicts(ctx context.Context) ([]ConflictFile, error) {
	paths := g.conflicts(ctx)
	result := make([]ConflictFile, 0, len(paths))
	for _, path := range paths {
		file := ConflictFile{Path: path}
		file.Ours = g.stage(ctx, 2, path)
		file.Theirs = g.stage(ctx, 3, path)
		if data, err := os.ReadFile(filepath.Join(g.repo, filepath.FromSlash(path))); err == nil {
			file.Both = string(data)
			file.Binary = strings.IndexByte(string(data), 0) >= 0
		}
		result = append(result, file)
	}
	return result, nil
}

func (g *Git) stage(ctx context.Context, stage int, path string) string {
	value, err := g.Run(ctx, "show", fmt.Sprintf(":%d:%s", stage, path))
	if err != nil {
		return ""
	}
	return string(value)
}

func (g *Git) Resolve(ctx context.Context, path, action, content string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}
	if action != "ours" && action != "theirs" && action != "content" {
		return errors.New("invalid conflict resolution")
	}
	if action == "content" {
		if strings.Contains(content, "<<<<<<<") || strings.Contains(content, "=======") || strings.Contains(content, ">>>>>>>") {
			return errors.New("conflict markers must be removed")
		}
		if err := os.WriteFile(filepath.Join(g.repo, filepath.FromSlash(path)), []byte(content), 0o600); err != nil {
			return err
		}
	} else if _, err := g.mutate(ctx, "checkout", "--"+action, "--", path); err != nil {
		return err
	}
	_, err := g.mutate(ctx, "add", "--", path)
	return err
}

func (g *Git) Continue(ctx context.Context, operation string) error {
	if operation == "rebase" {
		_, err := g.mutate(ctx, "rebase", "--continue")
		return err
	}
	_, err := g.mutate(ctx, "commit", "--no-edit")
	return err
}
