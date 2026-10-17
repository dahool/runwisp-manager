// SPDX-License-Identifier: GPL-3.0-or-later
//
// RunWisp Manager is the small HTTP service that hosts the manager UI.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"runtime"

	"github.com/runwisp/runwisp-manager/internal/httpx"
)

var (
	version = "dev"
	commit  = "unknown"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "--version" {
		fmt.Printf("runwisp-manager %s (%s) %s/%s\n", version, commit, runtime.GOOS, runtime.GOARCH)
		return nil
	}

	flags := flag.NewFlagSet("runwisp-manager", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	var address string
	flags.StringVar(&address, "listen", ":9478", "HTTP listen address")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() > 0 && flags.Arg(0) != "serve" {
		return fmt.Errorf("unknown command %q", flags.Arg(0))
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return httpx.Serve(address, version, commit, logger)
}
