// SPDX-License-Identifier: GPL-3.0-or-later
package validate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

type Shadow struct{ Dir, Root string }

// NewShadow creates a 0700 temporary tree. Candidate paths are written as
// regular files; every other existing file is symlinked to the source tree.
func NewShadow(ctx context.Context, root, stateDir string, candidates map[string][]byte) (Shadow, error) {
	if err := ctx.Err(); err != nil {
		return Shadow{}, err
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return Shadow{}, fmt.Errorf("resolve validation root: %w", err)
	}
	base, err := os.MkdirTemp(stateDir, ".validate-")
	if err != nil {
		return Shadow{}, fmt.Errorf("create validation shadow: %w", err)
	}
	clean := func() { _ = os.RemoveAll(base) }
	if err := mirror(filepath.Dir(root), base, root, candidates); err != nil {
		clean()
		return Shadow{}, err
	}
	shadow := Shadow{Dir: base, Root: filepath.Join(base, filepath.Base(root))}
	if done := ctx.Done(); done != nil {
		go func() { <-done; _ = os.RemoveAll(base) }()
	}
	return shadow, nil
}

func (s Shadow) Close() error {
	if s.Dir == "" {
		return nil
	}
	return os.RemoveAll(s.Dir)
}

func mirror(source, target, root string, candidates map[string][]byte) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("read validation source: %w", err)
	}
	for _, entry := range entries {
		src := filepath.Join(source, entry.Name())
		dst := filepath.Join(target, entry.Name())
		if entry.IsDir() {
			if err := os.Mkdir(dst, 0o700); err != nil {
				return err
			}
			if err := mirror(src, dst, root, candidates); err != nil {
				return err
			}
			continue
		}
		data, candidate := candidates[src]
		if candidate {
			if err := os.WriteFile(dst, data, 0o600); err != nil {
				return err
			}
			continue
		}
		if err := os.Symlink(src, dst); err != nil {
			return fmt.Errorf("link validation sibling %q: %w", src, err)
		}
	}
	return nil
}
