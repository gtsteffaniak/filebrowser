package indexing

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"

	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// These tests simulate an entry whose lstat fails (on Linux, a Windows junction seen through a CIFS
// mount whose target the share user cannot read) by injecting the error through the seams in
// readDirSkippingUnreadable. They cover the skip policy, not the kernel behaviour itself.

func lstatFixture(t *testing.T) (*Index, string) {
	t.Helper()
	if indexDB == nil {
		var err error
		indexDB, _, err = dbsql.NewIndexDB("test_indexing", "OFF", 1000, 32, false)
		if err != nil {
			t.Fatalf("Failed to create test database: %v", err)
		}
	}
	root := t.TempDir()
	for _, name := range []string{"ok.txt", "broken-junction"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(root, "okdir"), 0o755); err != nil {
		t.Fatal(err)
	}
	return &Index{Source: settings.Source{Name: "test_lstat", Path: root}, db: indexDB}, root
}

func listRoot(t *testing.T, idx *Index, root string) []string {
	t.Helper()
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	stat, err := dir.Stat()
	if err != nil {
		t.Fatal(err)
	}
	info, _, err := idx.GetDirInfoCore(dir, stat, "/", Options{SkipExtendedAttrs: true}, nil)
	if err != nil {
		t.Fatalf("listing failed because of one unreadable entry: %v", err)
	}
	var got []string
	for _, f := range info.Files {
		got = append(got, f.Name)
	}
	for _, d := range info.Folders {
		got = append(got, d.Name+"/")
	}
	sort.Strings(got)
	return got
}

func wantOkEntries(t *testing.T, got []string) {
	t.Helper()
	if len(got) != 2 || got[0] != "ok.txt" || got[1] != "okdir/" {
		t.Fatalf("listed %v, want [ok.txt okdir/]", got)
	}
}

func TestGetDirInfoCore_SkipsEntryWhoseInfoFails(t *testing.T) {
	idx, root := lstatFixture(t)
	orig := entryInfo
	t.Cleanup(func() { entryInfo = orig })
	entryInfo = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: os.ErrPermission}
		}
		return orig(e)
	}
	wantOkEntries(t, listRoot(t, idx, root))
}

func TestGetDirInfoCore_SkipsEntryRemovedBeforeStat(t *testing.T) {
	idx, root := lstatFixture(t)
	orig := entryInfo
	t.Cleanup(func() { entryInfo = orig })
	entryInfo = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: os.ErrNotExist}
		}
		return orig(e)
	}
	wantOkEntries(t, listRoot(t, idx, root))
}

// When ReadDir itself fails (it lstats on its own when d_type is unknown), the directory is re-read by
// name and the unreadable entry is skipped there too.
func TestGetDirInfoCore_FallsBackToNamesWhenReadDirFails(t *testing.T) {
	idx, root := lstatFixture(t)
	origRead, origLstat := readDirEntries, lstatPath
	t.Cleanup(func() { readDirEntries, lstatPath = origRead, origLstat })
	readDirEntries = func(f *os.File) ([]os.DirEntry, error) {
		// consume the directory, as a failing ReadDir would, so the fallback has to rewind
		_, _ = f.ReadDir(-1)
		return nil, &os.PathError{Op: "lstat", Path: "broken-junction", Err: os.ErrPermission}
	}
	lstatPath = func(name string) (os.FileInfo, error) {
		if filepath.Base(name) == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: name, Err: os.ErrPermission}
		}
		return origLstat(name)
	}
	wantOkEntries(t, listRoot(t, idx, root))
}

func listRootErr(t *testing.T, idx *Index, root string) error {
	t.Helper()
	dir, err := os.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	defer dir.Close()
	stat, err := dir.Stat()
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = idx.GetDirInfoCore(dir, stat, "/", Options{SkipExtendedAttrs: true}, nil)
	return err
}

var errIO = errors.New("input/output error")

// An I/O error on a network mount must fail the listing; skipping it would let the scanner prune
// files that still exist.
func TestGetDirInfoCore_FailsWhenEntryInfoHasIOError(t *testing.T) {
	idx, root := lstatFixture(t)
	orig := entryInfo
	t.Cleanup(func() { entryInfo = orig })
	entryInfo = func(e os.DirEntry) (os.FileInfo, error) {
		if e.Name() == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: e.Name(), Err: errIO}
		}
		return orig(e)
	}
	if err := listRootErr(t, idx, root); !errors.Is(err, errIO) {
		t.Fatalf("got %v, want the I/O error", err)
	}
}

func TestGetDirInfoCore_FailsWhenFallbackLstatHasIOError(t *testing.T) {
	idx, root := lstatFixture(t)
	origRead, origLstat := readDirEntries, lstatPath
	t.Cleanup(func() { readDirEntries, lstatPath = origRead, origLstat })
	readDirEntries = func(f *os.File) ([]os.DirEntry, error) {
		_, _ = f.ReadDir(-1)
		return nil, &os.PathError{Op: "lstat", Path: "broken-junction", Err: os.ErrPermission}
	}
	lstatPath = func(name string) (os.FileInfo, error) {
		if filepath.Base(name) == "broken-junction" {
			return nil, &os.PathError{Op: "lstat", Path: name, Err: errIO}
		}
		return origLstat(name)
	}
	if err := listRootErr(t, idx, root); !errors.Is(err, errIO) {
		t.Fatalf("got %v, want the I/O error", err)
	}
}

// An error reading the directory itself is not retried by name, even a permission error.
func TestGetDirInfoCore_FailsWhenDirectoryReadFails(t *testing.T) {
	idx, root := lstatFixture(t)
	orig := readDirEntries
	t.Cleanup(func() { readDirEntries = orig })
	readDirEntries = func(f *os.File) ([]os.DirEntry, error) {
		return nil, &os.PathError{Op: "readdirent", Path: f.Name(), Err: os.ErrPermission}
	}
	if err := listRootErr(t, idx, root); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("got %v, want the directory read error", err)
	}
}

// ReadDir's own lstat failing with an I/O error is not retried by name either.
func TestGetDirInfoCore_FailsWhenReadDirLstatHasIOError(t *testing.T) {
	idx, root := lstatFixture(t)
	orig := readDirEntries
	t.Cleanup(func() { readDirEntries = orig })
	readDirEntries = func(f *os.File) ([]os.DirEntry, error) {
		_, _ = f.ReadDir(-1)
		return nil, &os.PathError{Op: "lstat", Path: "broken-junction", Err: errIO}
	}
	if err := listRootErr(t, idx, root); !errors.Is(err, errIO) {
		t.Fatalf("got %v, want the I/O error", err)
	}
}

func warned(path string) bool {
	unreadableMu.Lock()
	defer unreadableMu.Unlock()
	_, seen := unreadableWarned[path]
	return seen
}

func warnedCount() int {
	unreadableMu.Lock()
	defer unreadableMu.Unlock()
	return len(unreadableWarned)
}

func boundWarnedSet(t *testing.T, max int) {
	t.Helper()
	origMax := maxUnreadableWarned
	t.Cleanup(func() { maxUnreadableWarned = origMax })
	unreadableMu.Lock()
	clear(unreadableWarned)
	maxUnreadableWarned = max
	unreadableMu.Unlock()
}

func TestSkipUnreadable_WarnedSetIsBounded(t *testing.T) {
	boundWarnedSet(t, 2)
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		skipUnreadable(filepath.Join(dir, name), os.ErrPermission)
	}
	if warned(filepath.Join(dir, "a")) || !warned(filepath.Join(dir, "c")) || warnedCount() != 1 {
		t.Fatalf("after 3 paths with bound 2, want only the last one recorded; have %d", warnedCount())
	}
}

func TestSkipUnreadable_WarnedSetStaysBoundedUnderConcurrency(t *testing.T) {
	boundWarnedSet(t, 50)
	dir := t.TempDir()
	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < 500; i++ {
				skipUnreadable(filepath.Join(dir, fmt.Sprintf("g%d-%d", g, i)), os.ErrPermission)
			}
		}(g)
	}
	wg.Wait()
	if n := warnedCount(); n > 50 {
		t.Fatalf("warned set holds %d paths, bound is 50", n)
	}
}

func TestSkipUnreadable_WarnsOncePerPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junction")
	skipUnreadable(path, os.ErrPermission)
	if !warned(path) {
		t.Fatal("first failure was not recorded")
	}
	if firstWarning(path) {
		t.Fatal("a second failure on the same path would warn again")
	}
	skipUnreadable(path+"-gone", &os.PathError{Op: "lstat", Path: path + "-gone", Err: os.ErrNotExist})
	if warned(path + "-gone") {
		t.Fatal("a vanished entry must not be recorded as unreadable")
	}
}
