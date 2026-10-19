//go:build linux

package indexing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadDirFileInfos_MatchesLstat(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	entries, err := readDirFileInfos(f)
	if err != nil {
		t.Fatal(err)
	}

	byName := make(map[string]os.FileInfo, len(entries))
	for _, e := range entries {
		byName[e.Name()] = e
	}

	for name := range byName {
		want, err := os.Lstat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		got := byName[name]
		if got.Mode() != want.Mode() {
			t.Errorf("%s: mode got %v want %v", name, got.Mode(), want.Mode())
		}
		if got.Size() != want.Size() {
			t.Errorf("%s: size got %d want %d", name, got.Size(), want.Size())
		}
		if !got.ModTime().Equal(want.ModTime()) {
			t.Errorf("%s: mtime got %v want %v", name, got.ModTime(), want.ModTime())
		}
		if got.IsDir() != want.IsDir() {
			t.Errorf("%s: isDir got %v want %v", name, got.IsDir(), want.IsDir())
		}
		if _, ok := got.(*statxFileInfo); !ok {
			t.Errorf("%s: expected statxFileInfo, got %T", name, got)
		}
	}
}

func TestReadDirFileInfos_CreatedTimeWithoutExtraPathStatx(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "created.txt")
	if err := os.WriteFile(path, []byte("z"), 0o644); err != nil {
		t.Fatal(err)
	}

	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	entries, err := readDirFileInfos(f)
	if err != nil {
		t.Fatal(err)
	}
	var fi os.FileInfo
	for _, e := range entries {
		if e.Name() == "created.txt" {
			fi = e
			break
		}
	}
	if fi == nil {
		t.Fatal("missing created.txt entry")
	}

	created := getCreatedTime(fi, "")
	if created == nil {
		t.Fatal("expected non-nil created time from listing stat")
	}
	sxi, ok := fi.(*statxFileInfo)
	if ok && !sxi.hasBtime && !created.Equal(fi.ModTime()) {
		t.Errorf("without STATX_BTIME, created should equal mtime: got %v mtime %v", created, fi.ModTime())
	}
}
