// SPDX-License-Identifier: GPL-3.0-or-later
package main

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/runwisp/runwisp-manager/internal/config"
)

func healthcheck(environ []string) error {
	c, err := config.Load(environ)
	if err != nil {
		return fmt.Errorf("load healthcheck configuration: %w", err)
	}
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return fmt.Errorf("parse RWM_LISTEN %q: %w", c.Listen, err)
	}
	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		host = "127.0.0.1"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return fmt.Errorf("parse RWM_LISTEN port %q: %w", port, err)
	}
	url := "http://" + net.JoinHostPort(host, port) + "/manager/healthz"
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("manager health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("manager health returned HTTP %d", response.StatusCode)
	}
	return nil
}

func healthcheckURL(listen string) string {
	if strings.TrimSpace(listen) == "" {
		return "http://127.0.0.1:9478/manager/healthz"
	}
	return "http://" + listen + "/manager/healthz"
}
