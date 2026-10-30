//go:build linux

package indexing

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func writeFixtureDir(t *testing.T) string {
	t.Helper()
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
	return dir
}

func TestReadDirFileInfos_MatchesLstat(t *testing.T) {
	dir := writeFixtureDir(t)

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

	wantNames := []string{"a.txt", "sub", "target", "link"}
	if len(byName) != len(wantNames) {
		t.Fatalf("expected %d entries, got %d", len(wantNames), len(byName))
	}

	for _, name := range wantNames {
		got, ok := byName[name]
		if !ok {
			t.Fatalf("missing entry %s", name)
		}
		want, err := os.Lstat(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
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
		// Concrete type must match the probed kernel support.
		if statxDirListingSupported {
			if _, ok := got.(*statxFileInfo); !ok {
				t.Errorf("%s: expected statxFileInfo, got %T", name, got)
			}
		} else if _, ok := got.(*fstatatFileInfo); !ok {
			t.Errorf("%s: expected fstatatFileInfo, got %T", name, got)
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

	// An empty realPath proves created time comes from the listing stat itself.
	created := getCreatedTime(fi, "")
	switch e := fi.(type) {
	case *statxFileInfo:
		if e.hasBtime {
			if created == nil {
				t.Error("expected non-nil created time when STATX_BTIME is present")
			}
		} else if created != nil {
			t.Errorf("expected nil created without STATX_BTIME, got %v", created)
		}
	case *fstatatFileInfo:
		if created != nil {
			t.Errorf("fstatat has no birth time, expected nil created, got %v", created)
		}
	default:
		t.Fatalf("unexpected FileInfo type %T", fi)
	}
}

func TestStatDirEntry_MissingName(t *testing.T) {
	dir := t.TempDir()
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	for _, useStatx := range []bool{true, false} {
		_, err := statDirEntry(int(f.Fd()), "does-not-exist", useStatx)
		if !os.IsNotExist(err) {
			t.Fatalf("useStatx=%v: expected IsNotExist error, got %v", useStatx, err)
		}
	}
}

func TestFileInfoIsDirFileTypeMask(t *testing.T) {
	cases := []struct {
		name string
		mode uint32
		want bool
	}{
		{"dir", unix.S_IFDIR | 0o755, true},
		{"regular", unix.S_IFREG | 0o644, false},
		{"symlink", unix.S_IFLNK | 0o777, false},
		{"socket", unix.S_IFSOCK | 0o777, false},
		{"blockdev", unix.S_IFBLK | 0o660, false},
		{"chardev", unix.S_IFCHR | 0o660, false},
		{"fifo", unix.S_IFIFO | 0o644, false},
	}
	for _, tc := range cases {
		sfi := &statxFileInfo{name: tc.name, mode: uint16(tc.mode)}
		if got := sfi.IsDir(); got != tc.want {
			t.Errorf("statxFileInfo %s mode %#o: IsDir=%v want %v", tc.name, tc.mode, got, tc.want)
		}
		ffi := &fstatatFileInfo{name: tc.name, stat: unix.Stat_t{Mode: tc.mode}}
		if got := ffi.IsDir(); got != tc.want {
			t.Errorf("fstatatFileInfo %s mode %#o: IsDir=%v want %v", tc.name, tc.mode, got, tc.want)
		}
	}
}

func TestGetFileDetails_StatxAndFstatat(t *testing.T) {
	sfi := &statxFileInfo{
		name:   "f",
		size:   4097,
		blocks: 16,
		nlink:  2,
		ino:    4242,
	}
	realSize, nlink, ino, ok := getFileDetails(sfi.Sys(), "", false)
	if !ok || realSize != 16*512 || nlink != 2 || ino != 4242 {
		t.Errorf("statx disk-usage details: got (%d,%d,%d,%v) want (8192,2,4242,true)", realSize, nlink, ino, ok)
	}
	realSize, nlink, ino, ok = getFileDetails(sfi.Sys(), "", true)
	if !ok || realSize != 4097 || nlink != 2 || ino != 4242 {
		t.Errorf("statx logical details: got (%d,%d,%d,%v) want (4097,2,4242,true)", realSize, nlink, ino, ok)
	}

	ffi := &fstatatFileInfo{
		name: "f",
		stat: unix.Stat_t{Size: 123, Blocks: 8, Nlink: 3, Ino: 77},
	}
	realSize, nlink, ino, ok = getFileDetails(ffi.Sys(), "", false)
	if !ok || realSize != 8*512 || nlink != 3 || ino != 77 {
		t.Errorf("fstatat disk-usage details: got (%d,%d,%d,%v) want (4096,3,77,true)", realSize, nlink, ino, ok)
	}
	realSize, nlink, ino, ok = getFileDetails(ffi.Sys(), "", true)
	if !ok || realSize != 123 || nlink != 3 || ino != 77 {
		t.Errorf("fstatat logical details: got (%d,%d,%d,%v) want (123,3,77,true)", realSize, nlink, ino, ok)
	}
}

func TestGetCreatedTime_Fallbacks(t *testing.T) {
	if got := getCreatedTime(&fstatatFileInfo{name: "f"}, ""); got != nil {
		t.Errorf("fstatatFileInfo: expected nil created, got %v", got)
	}
	noBtime := &statxFileInfo{name: "f", hasBtime: false}
	if got := getCreatedTime(noBtime, ""); got != nil {
		t.Errorf("statxFileInfo without BTIME: expected nil created, got %v", got)
	}
	withBtime := &statxFileInfo{
		name:     "f",
		hasBtime: true,
		btime:    unix.StatxTimestamp{Sec: 1700000000, Nsec: 42},
	}
	got := getCreatedTime(withBtime, "")
	if got == nil || !got.Equal(time.Unix(1700000000, 42)) {
		t.Errorf("statxFileInfo with BTIME: got %v", got)
	}
}

func TestReadDirFileInfos_Parallel(t *testing.T) {
	dir := t.TempDir()
	const count = dirStatParallelThreshold + 44
	for i := 0; i < count; i++ {
		name := filepath.Join(dir, fmt.Sprintf("file_%04d", i))
		if err := os.WriteFile(name, []byte{byte(i % 251)}, 0o644); err != nil {
			t.Fatal(err)
		}
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
	if len(entries) != count {
		t.Fatalf("expected %d entries, got %d", count, len(entries))
	}
	for _, e := range entries {
		want, err := os.Lstat(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if e.Size() != want.Size() || e.IsDir() != want.IsDir() || !e.ModTime().Equal(want.ModTime()) {
			t.Errorf("%s: mismatch vs lstat", e.Name())
		}
	}
}

// writeNumberedFiles creates count files in dir and returns their names.
func writeNumberedFiles(t *testing.T, dir string, count int) []string {
	t.Helper()
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("file_%04d", i)
		if err := os.WriteFile(filepath.Join(dir, name), []byte{byte(i % 251)}, 0o644); err != nil {
			t.Fatal(err)
		}
		names = append(names, name)
	}
	return names
}

func openTestDir(t *testing.T, dir string) *os.File {
	t.Helper()
	f, err := os.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

// injectStatErr makes the stat of entry bad fail with err, on both the serial and parallel paths.
func injectStatErr(t *testing.T, bad string, err error) {
	t.Helper()
	orig := statEntry
	t.Cleanup(func() { statEntry = orig })
	statEntry = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		if name == bad {
			return nil, &fs.PathError{Op: "fstatat", Path: name, Err: err}
		}
		return orig(dirFd, name, useStatx)
	}
}

// injectAllStatErr makes every entry's stat fail with err.
func injectAllStatErr(t *testing.T, err error) {
	t.Helper()
	orig := statEntry
	t.Cleanup(func() { statEntry = orig })
	statEntry = func(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
		return nil, &fs.PathError{Op: "fstatat", Path: name, Err: err}
	}
}

// listingSizes covers the serial path (below dirStatParallelThreshold) and the parallel path.
var listingSizes = []int{3, dirStatParallelThreshold + 44}

func TestReadDirFileInfos_SkipsPermissionDeniedEntry(t *testing.T) {
	for _, count := range listingSizes {
		t.Run(fmt.Sprintf("entries=%d", count), func(t *testing.T) {
			dir := t.TempDir()
			names := writeNumberedFiles(t, dir, count)
			injectStatErr(t, names[0], syscall.EACCES)

			infos, err := readDirFileInfos(openTestDir(t, dir))
			if err != nil {
				t.Fatalf("one EACCES entry failed the listing: %v", err)
			}
			if len(infos) != count-1 {
				t.Fatalf("listed %d entries, want %d", len(infos), count-1)
			}
			for _, fi := range infos {
				if fi.Name() == names[0] {
					t.Fatal("unreadable entry was listed")
				}
			}
		})
	}
}

func TestReadDirFileInfos_SkipsVanishedEntry(t *testing.T) {
	for _, count := range listingSizes {
		t.Run(fmt.Sprintf("entries=%d", count), func(t *testing.T) {
			dir := t.TempDir()
			names := writeNumberedFiles(t, dir, count)
			injectStatErr(t, names[0], syscall.ENOENT)

			infos, err := readDirFileInfos(openTestDir(t, dir))
			if err != nil {
				t.Fatalf("one vanished entry failed the listing: %v", err)
			}
			if len(infos) != count-1 {
				t.Fatalf("listed %d entries, want %d", len(infos), count-1)
			}
		})
	}
}

// EIO on a network mount must fail the listing; skipping it would let the scanner prune files that
// still exist.
func TestReadDirFileInfos_IOErrorFailsListing(t *testing.T) {
	for _, count := range listingSizes {
		t.Run(fmt.Sprintf("entries=%d", count), func(t *testing.T) {
			dir := t.TempDir()
			names := writeNumberedFiles(t, dir, count)
			injectStatErr(t, names[count/2], syscall.EIO)

			if _, err := readDirFileInfos(openTestDir(t, dir)); !errors.Is(err, syscall.EIO) {
				t.Fatalf("got %v, want EIO to fail the listing", err)
			}
		})
	}
}

func TestReadDirFileInfos_StaleHandleFailsListing(t *testing.T) {
	dir := t.TempDir()
	names := writeNumberedFiles(t, dir, 3)
	injectStatErr(t, names[1], syscall.ESTALE)

	if _, err := readDirFileInfos(openTestDir(t, dir)); !errors.Is(err, syscall.ESTALE) {
		t.Fatalf("got %v, want ESTALE to fail the listing", err)
	}
}

// If every entry vanished, the directory was removed or renamed mid-scan. An empty success would make
// the scanner prune all of its children.
func TestReadDirFileInfos_AllEntriesVanishedFails(t *testing.T) {
	for _, count := range listingSizes {
		t.Run(fmt.Sprintf("entries=%d", count), func(t *testing.T) {
			dir := t.TempDir()
			writeNumberedFiles(t, dir, count)
			injectAllStatErr(t, syscall.ENOENT)
			f := openTestDir(t, dir)
			// The directory itself is renamed away mid-scan; the open handle still lists it.
			if err := os.Rename(dir, dir+".moved"); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.RemoveAll(dir + ".moved") })

			if _, err := readDirFileInfos(f); !errors.Is(err, fs.ErrNotExist) {
				t.Fatalf("got %v, want a not-exist error for an all-vanished listing", err)
			}
		})
	}
}

// Every entry vanishing from a directory that still exists (short-lived temp files) is a valid
// empty listing, not a not-exist error for the directory.
func TestReadDirFileInfos_AllEntriesVanishedDirPresentIsEmpty(t *testing.T) {
	dir := t.TempDir()
	writeNumberedFiles(t, dir, 1)
	injectAllStatErr(t, syscall.ENOENT)

	infos, err := readDirFileInfos(openTestDir(t, dir))
	if err != nil {
		t.Fatalf("got %v, want an empty success", err)
	}
	if len(infos) != 0 {
		t.Fatalf("listed %d entries, want 0", len(infos))
	}
}

func TestReadDirFileInfos_EmptyDirIsNotAnError(t *testing.T) {
	infos, err := readDirFileInfos(openTestDir(t, t.TempDir()))
	if err != nil || len(infos) != 0 {
		t.Fatalf("empty dir: got %d entries, err %v", len(infos), err)
	}
}

// Real EACCES, no seam: a directory that is readable but not searchable lists its names, while each
// per-entry stat fails with EACCES. Root ignores directory permissions, so skip there.
func TestReadDirFileInfos_RealPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}
	dir := t.TempDir()
	writeNumberedFiles(t, dir, 2)
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Fatal(err)
	}
	// Registered after TempDir's cleanup, so it runs first and the directory can be removed.
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	infos, err := readDirFileInfos(openTestDir(t, dir))
	if err != nil {
		t.Fatalf("unsearchable directory failed the listing: %v", err)
	}
	if len(infos) != 0 {
		t.Fatalf("listed %d entries, want 0", len(infos))
	}
}

func TestCheckNotAllVanished(t *testing.T) {
	if err := checkNotAllVanished("/d", 0, 0); err != nil {
		t.Errorf("empty listing: got %v, want nil", err)
	}
	if err := checkNotAllVanished("/d", 3, 2); err != nil {
		t.Errorf("partial vanish: got %v, want nil", err)
	}
	gone := filepath.Join(t.TempDir(), "gone")
	if err := checkNotAllVanished(gone, 3, 3); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("all vanished, directory gone: got %v, want not-exist", err)
	}
	// A one-entry directory whose only entry was deleted mid-scan still exists: empty success.
	if err := checkNotAllVanished(t.TempDir(), 1, 1); err != nil {
		t.Errorf("all vanished, directory present: got %v, want nil", err)
	}
	// A stat failure other than not-exist (here ENOTDIR) must fail the listing, not report empty.
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := checkNotAllVanished(filepath.Join(file, "child"), 1, 1); err == nil {
		t.Error("all vanished, stat error: got nil, want error")
	}
}

func TestSkipEntry_Classification(t *testing.T) {
	dir := t.TempDir()
	if !skipEntry(dir, "perm", syscall.EACCES) {
		t.Error("EACCES should skip")
	}
	if !skipEntry(dir, "gone", syscall.ENOENT) {
		t.Error("ENOENT should skip")
	}
	if skipEntry(dir, "io", syscall.EIO) {
		t.Error("EIO must not skip")
	}
	if skipEntry(dir, "stale", syscall.ESTALE) {
		t.Error("ESTALE must not skip")
	}
	// A vanished entry is not recorded as unreadable.
	unreadableMu.Lock()
	_, recorded := unreadableWarned[filepath.Join(dir, "gone")]
	unreadableMu.Unlock()
	if recorded {
		t.Error("ENOENT must not be recorded in the warned set")
	}
}

func TestSkipEntry_WarnedSetIsBounded(t *testing.T) {
	origMax := maxUnreadableWarned
	t.Cleanup(func() {
		maxUnreadableWarned = origMax
		unreadableMu.Lock()
		clear(unreadableWarned)
		unreadableMu.Unlock()
	})
	maxUnreadableWarned = 2
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c"} {
		skipEntry(dir, name, syscall.EACCES)
	}
	unreadableMu.Lock()
	defer unreadableMu.Unlock()
	if len(unreadableWarned) > 2 {
		t.Fatalf("warned set holds %d paths, bound is 2", len(unreadableWarned))
	}
	if _, ok := unreadableWarned[filepath.Join(dir, "c")]; !ok {
		t.Error("latest path should be recorded after the set was cleared")
	}
}
