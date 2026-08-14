package iteminfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenContained_RejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	scopeDir := filepath.Join(root, "scope")
	if err := os.MkdirAll(scopeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(scopeDir, "escape")); err != nil {
		t.Fatal(err)
	}

	_, err := OpenContained(scopeDir, "escape")
	if err == nil {
		t.Fatal("expected symlink escape to be rejected")
	}
}

func TestOpenContained_OpensInScopeFile(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "allowed.txt")
	if err := os.WriteFile(inside, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	file, err := OpenContained(root, "allowed.txt")
	if err != nil {
		t.Fatalf("expected in-scope open to succeed: %v", err)
	}
	file.Close()
}
