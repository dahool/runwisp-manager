// SPDX-License-Identifier: GPL-3.0-or-later
// Package cfgsave implements the transactional configuration save pipeline.
package cfgsave

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/runwisp/runwisp-manager/internal/tomledit"
)

var (
	ErrBusy         = errors.New("configuration save is busy")
	ErrConflict     = errors.New("configuration changed since editing started")
	ErrEditDisabled = errors.New("editing is disabled")
	ErrManagedPath  = errors.New("file is outside the managed configuration area")
	ErrUnauthorized = errors.New("configuration change is not authorized")
)

// FileID identifies a managed file. It is the opaque ID from cfgtree, but
// paths are accepted by the pipeline's resolver to keep the package useful to
// callers that do not use the HTTP tree API.
type FileID string

// FileOps contains the surgical edits for one file.
type FileOps struct {
	File FileID        `json:"file"`
	Ops  []tomledit.Op `json:"ops"`
}

// FileContent is a complete replacement. Raw replacements are admin-only.
type FileContent struct {
	File    FileID `json:"file"`
	Content []byte `json:"content"`
}

// Change is the single request model for save and save-and-apply.
type Change struct {
	IdempotencyKey string            `json:"idempotencyKey"`
	Base           map[FileID]string `json:"base"`
	Ops            []FileOps         `json:"ops,omitempty"`
	Raw            []FileContent     `json:"raw,omitempty"`
	Apply          bool              `json:"apply"`
	Reason         string            `json:"reason,omitempty"`
}

// Conflict describes the current version and the change visible to the UI.
type Conflict struct {
	File        FileID `json:"file"`
	CurrentHash string `json:"currentHash"`
	UnifiedDiff string `json:"unifiedDiff"`
	ChangedBy   string `json:"changedBy,omitempty"`
}

// Authorizer is evaluated before a lock is acquired. It can return a reason
// suitable for the read-only response, while a non-nil error denies the edit.
type Authorizer func(context.Context, Change) (readOnlyReason string, err error)

// Resolver maps an opaque file ID to an absolute path in the managed tree.
type Resolver func(FileID) (string, error)

// Lock serializes saves in this process and across manager processes.
type Lock struct {
	mu sync.Mutex
}

func validateManaged(root, path string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("resolve managed root: %w", err)
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve managed file: %w", err)
	}
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return ErrManagedPath
	}
	return nil
}

func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read configuration file %q: %w", path, err)
	}
	return data, nil
}
