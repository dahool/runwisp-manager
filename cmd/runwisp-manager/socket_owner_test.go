// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckRunWispSocketOwner(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "runwisp.sock")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Skipf("Unix sockets unavailable: %v", err)
	}
	defer listener.Close()
	defer os.Remove(socket)

	if err := checkRunWispSocketOwner(socket, uint32(os.Geteuid())); err != nil {
		t.Fatalf("matching socket owner: %v", err)
	}
	wrongUID := uint32(os.Geteuid() + 1)
	err = checkRunWispSocketOwner(socket, wrongUID)
	if err == nil || !strings.Contains(err.Error(), "set RWM_UID") {
		t.Fatalf("mismatched owner error = %v", err)
	}
}

func TestCheckRunWispSocketOwnerAllowsStartupBeforeSocketExists(t *testing.T) {
	if err := checkRunWispSocketOwner(filepath.Join(t.TempDir(), "missing.sock"), uint32(os.Geteuid())); err != nil {
		t.Fatalf("missing socket during startup: %v", err)
	}
}
