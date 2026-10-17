package validate

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestShadowReplacesCandidatesAndLinksSiblings(t *testing.T) {
	rootDir := t.TempDir()
	root := filepath.Join(rootDir, "root.toml")
	sibling := filepath.Join(rootDir, "sibling.toml")
	if err := os.WriteFile(root, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sibling, []byte("sibling"), 0600); err != nil {
		t.Fatal(err)
	}
	shadow, err := NewShadow(context.Background(), root, t.TempDir(), map[string][]byte{root: []byte("candidate")})
	if err != nil {
		t.Fatal(err)
	}
	defer shadow.Close()
	got, err := os.ReadFile(shadow.Root)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "candidate" {
		t.Fatalf("root = %q", got)
	}
	info, err := os.Lstat(filepath.Join(shadow.Dir, "sibling.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("sibling was not symlinked")
	}
	original, err := os.ReadFile(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(original) != "original" {
		t.Fatal("source was modified")
	}
}
