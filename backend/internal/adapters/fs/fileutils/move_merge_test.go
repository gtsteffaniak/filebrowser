package fileutils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMoveFileMergeDirectories(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "src")
	dst := filepath.Join(root, "dst")

	files := map[string]string{
		filepath.Join(src, "new.txt"):      "new",
		filepath.Join(src, "conflict.txt"): "NEW",
		filepath.Join(dst, "old.txt"):      "old",
		filepath.Join(dst, "conflict.txt"): "OLD",
	}

	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := MoveFileMerge(src, dst); err != nil {
		t.Fatal(err)
	}

	expected := map[string]string{
		"old.txt":      "old",
		"new.txt":      "new",
		"conflict.txt": "NEW",
	}

	for name, want := range expected {
		data, err := os.ReadFile(filepath.Join(dst, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != want {
			t.Errorf("%s = %q, want %q", name, data, want)
		}
	}

	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Errorf("source directory still exists: %v", err)
	}
}
