// SPDX-License-Identifier: GPL-3.0-or-later
package gitx

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

var ErrUndoHeadChanged = errors.New("pull result changed; undo refused")

// UndoPull restores ORIG_HEAD only when the repository is still at the
// recorded pull result and has no new work. It never accepts a ref or option
// from the caller.
func (g *Git) UndoPull(ctx context.Context, origHead, pulledHead string) error {
	if !validSHA(origHead) || !validSHA(pulledHead) {
		return errors.New("invalid pull undo head")
	}
	head, err := g.Run(ctx, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(head)) != pulledHead {
		return ErrUndoHeadChanged
	}
	if status, err := g.Run(ctx, "status", "--porcelain"); err != nil {
		return err
	} else if strings.TrimSpace(string(status)) != "" {
		return fmt.Errorf("%w: working tree is not clean", ErrUndoHeadChanged)
	}
	_, err = g.mutate(ctx, "reset", "--hard", origHead)
	return err
}

func validSHA(value string) bool {
	if len(value) != 40 {
		return false
	}
	for _, char := range value {
		if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f' || char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}
