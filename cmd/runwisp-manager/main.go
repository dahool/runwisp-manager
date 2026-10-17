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
	"strings"
	"time"

	"github.com/runwisp/runwisp-manager/internal/audit"
	"github.com/runwisp/runwisp-manager/internal/auth"
	"github.com/runwisp/runwisp-manager/internal/config"
	"github.com/runwisp/runwisp-manager/internal/daemon"
	"github.com/runwisp/runwisp-manager/internal/gitx"
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
	if os.Getenv("RWM_GIT_ASKPASS_TOKEN") != "" && len(args) > 0 {
		if strings.Contains(strings.ToLower(args[0]), "username") {
			value := os.Getenv("RWM_GIT_ASKPASS_USERNAME")
			if value == "" {
				value = "x-access-token"
			}
			fmt.Println(value)
		} else {
			fmt.Println(os.Getenv("RWM_GIT_ASKPASS_TOKEN"))
		}
		return nil
	}
	if len(args) > 0 && args[0] == "git-askpass" {
		prompt := os.Getenv("RWM_GIT_ASKPASS_PROMPT")
		if len(args) > 1 {
			prompt = args[1]
		}
		if prompt = strings.ToLower(strings.TrimSpace(prompt)); strings.Contains(prompt, "username") {
			value := os.Getenv("RWM_GIT_ASKPASS_USERNAME")
			if value == "" {
				value = "x-access-token"
			}
			fmt.Println(value)
			return nil
		}
		fmt.Println(os.Getenv("RWM_GIT_ASKPASS_TOKEN"))
		return nil
	}
	if len(args) > 0 && args[0] == "--version" {
		fmt.Printf("runwisp-manager %s (%s) %s/%s\n", version, commit, runtime.GOOS, runtime.GOARCH)
		return nil
	}
	if len(args) > 0 && args[0] == "check-config" {
		c, err := config.Load(os.Environ())
		if err != nil {
			return err
		}
		if problems := c.ValidateAuth(); len(problems) > 0 {
			return fmt.Errorf("authentication configuration invalid: %s", strings.Join(problems, "; "))
		}
		if c.AuthMode == "oidc" {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			_, err := auth.Discover(ctx, c)
			cancel()
			if err != nil {
				return fmt.Errorf("oidc discovery check failed: %w", err)
			}
		}
		if c.GitEnabled != "false" && c.GitRepo != "" {
			timeout, parseErr := time.ParseDuration(c.GitTimeout)
			if parseErr != nil {
				return parseErr
			}
			gitRunner, newErr := gitx.New(gitx.Options{Repo: c.GitRepo, Bin: c.GitBin, StateDir: c.StateDir, Timeout: timeout, AllowHooks: c.GitAllowHooks, Remote: c.GitRemote, Username: c.GitUsername, Token: c.GitToken, SSHKeyFile: c.GitSSHKeyFile, KnownHosts: c.GitSSHKnownHostsFile, AcceptNew: c.GitSSHInsecureAcceptNew})
			if newErr != nil {
				return fmt.Errorf("git configuration invalid: %w", newErr)
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			newErr = gitRunner.CheckRemote(ctx, c.GitRemote)
			cancel()
			if newErr != nil {
				return newErr
			}
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
	if problems := c.ValidateAuth(); len(problems) > 0 {
		return fmt.Errorf("authentication configuration invalid: %s", strings.Join(problems, "; "))
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
	var provider *auth.Provider
	var store *auth.Store
	var auditStore *audit.Store
	if c.AuthMode == "oidc" {
		provider, err = auth.Discover(context.Background(), c)
		if err != nil {
			return err
		}
		store, err = auth.OpenStore(context.Background(), c.StateDir, []byte(c.SessionSecret))
		if err != nil {
			return err
		}
		defer store.Close()
		go store.Janitor(context.Background(), 5*time.Minute)
	}
	if store != nil {
		auditStore, err = audit.OpenStoreDB(context.Background(), store.DB())
	} else {
		auditStore, err = audit.OpenStore(context.Background(), c.StateDir)
	}
	if err != nil {
		return err
	}
	defer auditStore.Close()
	auditStore.SetMirror(func(event audit.Event) { audit.SlogSink{Logger: logger}.Emit(context.Background(), event) })
	authentication := auth.NewAuthenticator(c, store, provider)
	authentication.SetAudit(auditStore)
	return httpx.ServeWithAuthAudit(address, version, commit, logger, discovery, networks, authentication, auditStore)
}
