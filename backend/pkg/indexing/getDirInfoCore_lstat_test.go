package indexing

import (
	"os"
	"path/filepath"
	"sort"
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
	info, err := idx.GetDirInfoCore(dir, stat, "/", Options{SkipExtendedAttrs: true}, nil)
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

func TestSkipUnreadable_WarnsOncePerPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "junction")
	skipUnreadable(path, os.ErrPermission)
	if _, seen := unreadableWarned.Load(path); !seen {
		t.Fatal("first failure was not recorded")
	}
	skipUnreadable(path+"-gone", &os.PathError{Op: "lstat", Path: path + "-gone", Err: os.ErrNotExist})
	if _, seen := unreadableWarned.Load(path + "-gone"); seen {
		t.Fatal("a vanished entry must not be recorded as unreadable")
	}
}
