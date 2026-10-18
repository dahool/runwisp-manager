//go:build windows

package cfgsave

import "os"

func preserveOwnership(string, os.FileInfo) error { return nil }
