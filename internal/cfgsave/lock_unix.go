//go:build linux

package cfgsave

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

func (l *Lock) Acquire(ctx context.Context, stateDir string, timeout time.Duration) (func(), error) {
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	deadline := time.Now().Add(timeout)
	for !l.mu.TryLock() {
		if err := waitLock(ctx, deadline); err != nil {
			return nil, err
		}
	}
	unlockFile, err := acquireFileLock(ctx, stateDir, deadline)
	if err != nil {
		l.mu.Unlock()
		return nil, err
	}
	return func() { unlockFile(); l.mu.Unlock() }, nil
}

func acquireFileLock(ctx context.Context, stateDir string, deadline time.Time) (func(), error) {
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(stateDir, "save.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	for {
		err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return func() { _ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN); _ = f.Close() }, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EAGAIN) {
			_ = f.Close()
			return nil, err
		}
		if err := waitLock(ctx, deadline); err != nil {
			_ = f.Close()
			return nil, err
		}
	}
}

func waitLock(ctx context.Context, deadline time.Time) error {
	if !time.Now().Before(deadline) {
		return ErrBusy
	}
	timer := time.NewTimer(10 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
