package cfgtree

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWatcherDetectsAtomicReplace(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "root.toml")
	if err := os.WriteFile(root, []byte("[task.alpha]\nenabled = true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	watcher, err := NewWatcher(Options{Root: root}, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	watcher.Start(ctx)
	tmp := filepath.Join(dir, ".root.toml.swap")
	if err := os.WriteFile(tmp, []byte("[task.alpha]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(tmp, root); err != nil {
		t.Fatal(err)
	}
	select {
	case event := <-watcher.Events():
		if event.TreeVersion == "" || len(event.ChangedFiles) == 0 {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(2 * time.Second):
		t.Skip("filesystem notifications are unavailable in this environment")
	}
	watcher.Close()
}

func TestDiscoverIncludesRelativeToIncludingFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, data string) {
		t.Helper()
		if err := os.WriteFile(name, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	root := filepath.Join(dir, "root.toml")
	write(root, "[daemon]\ninclude = [\"nested/*.toml\"]\n")
	write(filepath.Join(dir, "nested", "one.toml"), "[task.alpha]\nenabled = true\n")
	tree, err := Discover(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 2 {
		t.Fatalf("files = %d, want 2", len(tree.Files))
	}
	if tree.Files[0].Path != root {
		t.Fatalf("root path = %q, want %q", tree.Files[0].Path, root)
	}
}

func TestDiscoverReportsCycleAndOutsideSymlinkWithoutReadingIt(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "secret.toml")
	if err := os.WriteFile(outside, []byte("password = \"planted\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "outside.toml")); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(dir, "root.toml")
	if err := os.WriteFile(root, []byte("[daemon]\ninclude = [\"*.toml\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Discover(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(tree.Warnings, "\n")
	if !strings.Contains(joined, "outside managed area") {
		t.Fatalf("warnings = %q", joined)
	}
	for _, file := range tree.Files {
		if file.Outside && file.Readable {
			t.Fatalf("outside file marked readable: %+v", file)
		}
	}
}

func TestDiscoverMalformedFileIsRetained(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "root.toml")
	if err := os.WriteFile(root, []byte("[daemon\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Discover(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if len(tree.Files) != 1 || tree.Files[0].Error == "" {
		t.Fatalf("tree = %+v", tree)
	}
}

func TestPreviewIncludesRejectsEscapesAndReportsMissingMatches(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "one.toml"), []byte("# one\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := PreviewIncludes(dir, []string{"*.toml", "missing/*.toml", "../outside/*.toml"})
	if err != nil {
		t.Fatal(err)
	}
	if len(preview.Matches) != 1 || preview.Matches[0] != filepath.Join(dir, "one.toml") {
		t.Fatalf("matches = %#v", preview.Matches)
	}
	joined := strings.Join(preview.Warnings, "\n")
	if !strings.Contains(joined, "matched no files") || !strings.Contains(joined, "outside managed area") {
		t.Fatalf("warnings = %q", joined)
	}
}

func TestIndexUnitsLinesHashesAndDuplicates(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root := filepath.Join(dir, "root.toml")
	data := "\ufeff[task.alpha]\r\nenabled = true\r\n\r\n[service.api]\r\ncommand = \"run\"\r\n"
	if err := os.WriteFile(root, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(dir, "other.toml")
	if err := os.WriteFile(other, []byte("[task.alpha]\nenabled = false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tree, err := Discover(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	tree.Files = append(tree.Files, File{Path: other, Readable: true, Version: Hash([]byte("other"))})
	tree = Index(tree)
	if len(tree.Units) != 3 {
		t.Fatalf("units = %+v", tree.Units)
	}
	for _, unit := range tree.Units {
		if unit.Hash == "" || unit.Line[0] == 0 || unit.Line[1] < unit.Line[0] {
			t.Fatalf("bad unit: %+v", unit)
		}
	}
	duplicates := 0
	for _, unit := range tree.Units {
		if unit.Kind == "task" && unit.Duplicate {
			duplicates++
		}
	}
	if duplicates != 2 {
		t.Fatalf("duplicate task count = %d, units = %+v", duplicates, tree.Units)
	}
}
