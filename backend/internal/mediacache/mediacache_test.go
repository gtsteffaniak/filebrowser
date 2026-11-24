package mediacache

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestStatKeyChangesWithSizeAndModTime checks on disk that a size change
// or an mtime change gives a new key. The mtimes differ by whole seconds so
// the check holds on filesystems with coarse timestamps; the 1 ns case is
// covered by TestFromFileInfoNanosecondModTime.
func TestStatKeyChangesWithSizeAndModTime(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.mp4")
	if err := os.WriteFile(path, []byte("one"), 0o600); err != nil {
		t.Fatal(err)
	}
	k1, err := Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if k1.Path != path || k1.Size != 3 || k1.ModTime == 0 {
		t.Fatalf("unexpected key %+v", k1)
	}

	// Same size, different mtime.
	mt := time.Unix(1_700_000_000, 0)
	if err = os.Chtimes(path, mt, mt); err != nil {
		t.Fatal(err)
	}
	k2, err := Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if k2.String() == k1.String() {
		t.Fatalf("mtime change kept key %s", k2)
	}
	mt2 := mt.Add(time.Second)
	if err = os.Chtimes(path, mt2, mt2); err != nil {
		t.Fatal(err)
	}
	k3, _ := Stat(path)
	if k3.String() == k2.String() {
		t.Fatalf("1s mtime change kept key %s", k3)
	}

	// Same mtime, different size.
	if err = os.WriteFile(path, []byte("three"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err = os.Chtimes(path, mt2, mt2); err != nil {
		t.Fatal(err)
	}
	k4, _ := Stat(path)
	if k4.String() == k3.String() {
		t.Fatalf("size change kept key %s", k4)
	}
}

// stubFileInfo is an fs.FileInfo with a fixed size and mtime.
type stubFileInfo struct {
	size    int64
	modTime time.Time
}

// Name implements fs.FileInfo.
func (s stubFileInfo) Name() string { return "a.mp4" }

// Size implements fs.FileInfo.
func (s stubFileInfo) Size() int64 { return s.size }

// Mode implements fs.FileInfo.
func (s stubFileInfo) Mode() fs.FileMode { return 0o600 }

// ModTime implements fs.FileInfo.
func (s stubFileInfo) ModTime() time.Time { return s.modTime }

// IsDir implements fs.FileInfo.
func (s stubFileInfo) IsDir() bool { return false }

// Sys implements fs.FileInfo.
func (s stubFileInfo) Sys() any { return nil }

// TestFromFileInfoNanosecondModTime checks that mtimes 1 ns apart give
// different keys, independent of what a filesystem can store.
func TestFromFileInfoNanosecondModTime(t *testing.T) {
	a := FromFileInfo("/m/a.mp4", stubFileInfo{size: 3, modTime: time.Unix(1_700_000_000, 123)})
	b := FromFileInfo("/m/a.mp4", stubFileInfo{size: 3, modTime: time.Unix(1_700_000_000, 124)})
	if a.String() == b.String() {
		t.Fatalf("1ns mtime change kept key %s", a)
	}
	if a.ModTime != time.Unix(1_700_000_000, 123).UnixNano() {
		t.Fatalf("ModTime = %d, want nanoseconds", a.ModTime)
	}
}

// TestStatMissingFile checks that Stat reports a missing file.
func TestStatMissingFile(t *testing.T) {
	if _, err := Stat(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected an error for a missing file")
	}
}

// TestKeyStringDoesNotCollide checks that keys of different files, and of
// different streams of one file, never collide.
func TestKeyStringDoesNotCollide(t *testing.T) {
	a := Key{Path: "a:1", Size: 2, ModTime: 3}
	b := Key{Path: "a", Size: 1, ModTime: 2}
	if a.String() == b.String() {
		t.Fatalf("keys collide: %s", a)
	}
	if a.Stream(1) == a.Stream(2) || a.Stream(1) == a.String() {
		t.Fatalf("stream keys collide: %s %s", a.Stream(1), a.Stream(2))
	}
}

// TestClearAll checks that ClearAll empties every shared cache.
func TestClearAll(t *testing.T) {
	k := Key{Path: "/clear", Size: 1, ModTime: 1}
	Probes.Set(k.String(), ProbeInfo{Duration: 1})
	Durations.Set(k.String(), 1)
	SubtitleContent.Set(k.Stream(0), "x")
	ClearAll()
	if _, ok := Probes.Get(k.String()); ok {
		t.Fatal("probe survived ClearAll")
	}
	if _, ok := Durations.Get(k.String()); ok {
		t.Fatal("duration survived ClearAll")
	}
	if _, ok := SubtitleContent.Get(k.Stream(0)); ok {
		t.Fatal("subtitle content survived ClearAll")
	}
}

// TestWithoutCache checks the cache bypass marker and that derived contexts
// keep it.
func TestWithoutCache(t *testing.T) {
	if Bypassed(context.Background()) {
		t.Fatal("background context must not bypass")
	}
	//nolint:staticcheck // nil is part of the documented contract.
	if Bypassed(nil) {
		t.Fatal("nil context must not bypass")
	}
	ctx := WithoutCache(context.Background())
	if !Bypassed(ctx) {
		t.Fatal("WithoutCache context must bypass")
	}
	child, cancel := context.WithCancel(ctx)
	defer cancel()
	if !Bypassed(child) {
		t.Fatal("bypass must survive derived contexts")
	}
}
