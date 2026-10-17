// SPDX-License-Identifier: GPL-3.0-or-later
package main

import "testing"

func TestHealthcheckURL(t *testing.T) {
	if got := healthcheckURL("127.0.0.1:9478"); got != "http://127.0.0.1:9478/manager/healthz" {
		t.Fatalf("healthcheck URL = %q", got)
	}
}
