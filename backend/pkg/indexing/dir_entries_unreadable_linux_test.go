//go:build linux

package indexing

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/unix"
)

func TestErrnoToStatErr(t *testing.T) {
	if !errors.Is(errnoToStatErr(unix.EACCES), fs.ErrPermission) {
		t.Fatal("EACCES should map to fs.ErrPermission")
	}
}

func TestReadDirFileInfos_SkipsPermissionDeniedEntry(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"ok.txt", "broken-junction"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "okdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	orig := statDirEntryFn
	t.Cleanup(func() { statDirEntryFn = orig })
	statDirEntryFn = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		if name == "broken-junction" {
			return nil, fs.ErrPermission
		}
		return orig(dirFd, name, useStatx)
	}

	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	entries, err := readDirFileInfos(f)
	if err != nil {
		t.Fatalf("listing failed: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("listed %d entries, want 2", len(entries))
	}
}

func TestReadDirFileInfos_FailsOnIOErrorSerial(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "broken-junction"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	errIO := errors.New("input/output error")
	orig := statDirEntryFn
	t.Cleanup(func() { statDirEntryFn = orig })
	statDirEntryFn = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		if name == "broken-junction" {
			return nil, errIO
		}
		return orig(dirFd, name, useStatx)
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = readDirFileInfos(f)
	if !errors.Is(err, errIO) {
		t.Fatalf("got %v, want I/O error", err)
	}
}

func TestReadDirFileInfos_FailsOnIOErrorParallel(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < dirStatParallelThreshold+2; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file_%04d", i))
		if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "broken-junction"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	errIO := errors.New("input/output error")
	orig := statDirEntryFn
	t.Cleanup(func() { statDirEntryFn = orig })
	statDirEntryFn = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		if name == "broken-junction" {
			return nil, errIO
		}
		return orig(dirFd, name, useStatx)
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = readDirFileInfos(f)
	if !errors.Is(err, errIO) {
		t.Fatalf("got %v, want I/O error", err)
	}
}

func TestReadDirFileInfos_FailsWhenAllEntriesVanished(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := statDirEntryFn
	t.Cleanup(func() { statDirEntryFn = orig })
	statDirEntryFn = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		return nil, fs.ErrNotExist
	}
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	_, err = readDirFileInfos(f)
	if !errors.Is(err, errAllEntriesVanished) {
		t.Fatalf("got %v, want errAllEntriesVanished", err)
	}
}
