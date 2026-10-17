package gitx

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestDetectAndStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is unavailable")
	}
	root := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v (%s)", args, err, output)
		}
	}
	run("init", "-q")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.test")
	file := filepath.Join(root, "config.toml")
	if err := os.WriteFile(file, []byte("[daemon]\nport = 1\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	run("add", "--", "config.toml")
	run("commit", "-qm", "initial")
	if err := os.WriteFile(file, []byte("[daemon]\nport = 2\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	repo, g, err := Detect(context.Background(), root, "", Options{StateDir: filepath.Join(root, "state")})
	if err != nil || !repo.Enabled {
		t.Fatalf("Detect() = %#v, %v", repo, err)
	}
	status, err := g.Status(context.Background(), repo, []ManagedPath{{ID: "file-1", Path: file}})
	if err != nil {
		t.Fatal(err)
	}
	if len(status.Files) != 1 || !status.Files[0].Unstaged || status.Files[0].FileID != "file-1" {
		t.Fatalf("unexpected status: %#v", status)
	}
}
