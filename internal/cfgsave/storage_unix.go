//go:build unix

package cfgsave

import (
	"fmt"
	"os"
	"syscall"
)

func preserveOwnership(path string, info os.FileInfo) error {
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	if err := os.Chown(path, int(stat.Uid), int(stat.Gid)); err != nil {
		return fmt.Errorf("preserve target ownership: %w", err)
	}
	return nil
}
