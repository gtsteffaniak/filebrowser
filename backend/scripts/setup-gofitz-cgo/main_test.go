package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyFile(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	src := filepath.Join(srcDir, "header.h")
	if err := os.WriteFile(src, []byte("test content"), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	dst := filepath.Join(dstDir, "nested", "header.h")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read destination: %v", err)
	}
	if string(got) != "test content" {
		t.Fatalf("content = %q, want %q", string(got), "test content")
	}
}

func TestCopyDir_nestedFiles(t *testing.T) {
	t.Parallel()

	srcDir := t.TempDir()
	dstDir := t.TempDir()

	nested := filepath.Join(srcDir, "mupdf")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nested, "fitz.h"), []byte("header"), 0o644); err != nil {
		t.Fatalf("write nested: %v", err)
	}

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	root, err := os.ReadFile(filepath.Join(dstDir, "root.txt"))
	if err != nil {
		t.Fatalf("read root copy: %v", err)
	}
	if string(root) != "root" {
		t.Fatalf("root content = %q, want root", string(root))
	}

	nestedCopy, err := os.ReadFile(filepath.Join(dstDir, "mupdf", "fitz.h"))
	if err != nil {
		t.Fatalf("read nested copy: %v", err)
	}
	if string(nestedCopy) != "header" {
		t.Fatalf("nested content = %q, want header", string(nestedCopy))
	}
}

func TestRemoveExisting_directory(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	target := filepath.Join(dir, "gofitzinclude")
	if err := os.MkdirAll(filepath.Join(target, "mupdf"), 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}
	if err := os.WriteFile(filepath.Join(target, "mupdf", "fitz.h"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	if err := removeExisting(target); err != nil {
		t.Fatalf("removeExisting: %v", err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("target still exists after removeExisting: %v", err)
	}
}

func TestRemoveExisting_symlink(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	src := filepath.Join(dir, "include")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatalf("mkdir src: %v", err)
	}

	link := filepath.Join(dir, "gofitzinclude")
	if err := os.Symlink(src, link); err != nil {
		t.Skipf("symlinks not supported: %v", err)
	}

	if err := removeExisting(link); err != nil {
		t.Fatalf("removeExisting symlink: %v", err)
	}
	if _, err := os.Lstat(link); !os.IsNotExist(err) {
		t.Fatalf("symlink still exists: %v", err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Fatalf("symlink target removed: %v", err)
	}
}

func TestRemoveExisting_missingPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	if err := removeExisting(missing); err != nil {
		t.Fatalf("removeExisting missing: %v", err)
	}
}
