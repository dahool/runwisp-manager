//go:build !linux

package cfgsave

import (
	"context"
	"time"
)

func (l *Lock) Acquire(ctx context.Context, _ string, timeout time.Duration) (func(), error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for !l.mu.TryLock() {
		if !time.Now().Before(deadline) {
			return nil, ErrBusy
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
	return l.mu.Unlock, nil
}
