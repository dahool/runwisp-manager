package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuthorFallbackAndUnicode(t *testing.T) {
	got, err := Author(Actor{Username: "jdoe", Name: "José Niño"}, "example.test")
	if err != nil || got != "José Niño <jdoe@example.test>" {
		t.Fatalf("Author() = %q, %v", got, err)
	}
}

func TestCommitSelectedFileDisablesHooks(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.test")
	path := filepath.Join(root, "config.toml")
	if err := os.WriteFile(path, []byte("port = 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "config.toml")
	run("commit", "-qm", "initial")
	if err := os.WriteFile(path, []byte("port = 2\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	hook := filepath.Join(root, ".git", "hooks", "pre-commit")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\ntouch 'hook-ran'\nexit 1\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	g, err := New(Options{Repo: root, StateDir: filepath.Join(root, "state")})
	if err != nil {
		t.Fatal(err)
	}
	result, err := g.Commit(context.Background(), CommitRequest{Paths: []string{"config.toml"}, Message: "update config", Actor: Actor{Username: "user", Name: "User", Email: "user@example.test"}}, CommitOptions{})
	if err != nil || result.SHA == "" {
		t.Fatalf("Commit() = %#v, %v", result, err)
	}
	if _, err := os.Stat(filepath.Join(root, "hook-ran")); err == nil {
		t.Fatal("pre-commit hook executed")
	}
	log, err := g.Run(context.Background(), "log", "-1", "--format=%B")
	if err != nil || !strings.Contains(string(log), "Managed-By: runwisp-manager") {
		t.Fatalf("commit trailer missing: %q (%v)", log, err)
	}
}
