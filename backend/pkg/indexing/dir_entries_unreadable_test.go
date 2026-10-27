//go:build !linux

package indexing

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func unreadableFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"ok.txt", "broken-junction"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "okdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func listDir(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	f, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	entries, err := readDirFileInfos(f)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names, nil
}

func TestReadDirFileInfos_SkipsPermissionDeniedEntry(t *testing.T) {
	dir := unreadableFixture(t)
	orig := dirEntryInfoFn
	t.Cleanup(func() { dirEntryInfoFn = orig })
	dirEntryInfoFn = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: fs.ErrPermission}
		}
		return orig(e)
	}
	names, err := listDir(t, dir)
	if err != nil {
		t.Fatalf("listing failed because of one unreadable entry: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("listed %v, want ok.txt and okdir", names)
	}
}

func TestReadDirFileInfos_FailsOnIOError(t *testing.T) {
	dir := unreadableFixture(t)
	errIO := errors.New("input/output error")
	orig := dirEntryInfoFn
	t.Cleanup(func() { dirEntryInfoFn = orig })
	dirEntryInfoFn = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: errIO}
		}
		return orig(e)
	}
	_, err := listDir(t, dir)
	if !errors.Is(err, errIO) {
		t.Fatalf("got %v, want I/O error", err)
	}
}

func TestReadDirFileInfos_FailsWhenAllEntriesVanished(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "gone.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := dirEntryInfoFn
	t.Cleanup(func() { dirEntryInfoFn = orig })
	dirEntryInfoFn = func(e os.DirEntry) (os.FileInfo, error) {
		return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: fs.ErrNotExist}
	}
	_, err := listDir(t, dir)
	if !errors.Is(err, errAllEntriesVanished) {
		t.Fatalf("got %v, want errAllEntriesVanished", err)
	}
}

func TestReadDirFileInfos_ParallelSkipsPermission(t *testing.T) {
	dir := t.TempDir()
	const count = 256 + 10
	for i := 0; i < count; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file_%04d", i))
		if err := os.WriteFile(name, []byte{byte(i % 251)}, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "broken-junction"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	orig := dirEntryInfoFn
	t.Cleanup(func() { dirEntryInfoFn = orig })
	dirEntryInfoFn = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: fs.ErrPermission}
		}
		return orig(e)
	}
	names, err := listDir(t, dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != count {
		t.Fatalf("expected %d entries, got %d", count, len(names))
	}
}
