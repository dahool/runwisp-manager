// SPDX-License-Identifier: GPL-3.0-or-later
//
// RunWisp Manager is the small HTTP service that hosts the manager UI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/runwisp/runwisp-manager/internal/config"
	"github.com/runwisp/runwisp-manager/internal/daemon"
	"github.com/runwisp/runwisp-manager/internal/httpx"
	"github.com/runwisp/runwisp-manager/internal/runwispbin"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		if len(os.Args) > 1 && os.Args[1] == "check-config" {
			os.Exit(2)
		}
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "--version" {
		fmt.Printf("runwisp-manager %s (%s) %s/%s\n", version, commit, runtime.GOOS, runtime.GOARCH)
		return nil
	}
	if len(args) > 0 && args[0] == "check-config" {
		c, err := config.Load(os.Environ())
		if err != nil {
			return err
		}
		data, err := json.MarshalIndent(c.Effective(), "", "  ")
		if err != nil {
			return fmt.Errorf("format effective config: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	flags := flag.NewFlagSet("runwisp-manager", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var address string
	flags.StringVar(&address, "listen", "", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 && flags.Arg(0) != "serve" {
		return fmt.Errorf("unknown command %q", flags.Arg(0))
	}

	c, err := config.Load(os.Environ())
	if err != nil {
		return err
	}
	if address == "" {
		address = c.Listen
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	client, err := daemon.New(c, logger)
	if err != nil {
		return err
	}
	binaryVersion, _ := (runwispbin.Runner{Binary: c.RunWispBin, Env: os.Environ()}).Version(context.Background())
	discovery := &daemon.Discovery{Client: client, Config: c, BinaryVersion: binaryVersion}
	_ = discovery.Refresh(context.Background())
	discovery.Start(context.Background())
	networks, err := c.TrustedProxyNetworks()
	if err != nil {
		return err
	}
	return httpx.ServeWithDiscovery(address, version, commit, logger, discovery, networks)
}
