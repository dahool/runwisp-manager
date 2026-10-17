// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

func checkRunWispSocketOwner(path string, managerUID uint32) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil // RunWisp may still be creating its socket during startup.
	}
	if err != nil {
		return fmt.Errorf("inspect RunWisp socket %s: %w", path, err)
	}
	if info.Mode()&os.ModeSocket == 0 {
		return fmt.Errorf("RunWisp socket path %s is not a Unix socket", path)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("inspect RunWisp socket %s: ownership metadata is unavailable", path)
	}
	if stat.Uid != managerUID {
		return fmt.Errorf("RunWisp socket %s is owned by UID %d but manager runs as UID %d; set RWM_UID and both compose user values to the daemon UID", path, stat.Uid, managerUID)
	}
	return nil
}
