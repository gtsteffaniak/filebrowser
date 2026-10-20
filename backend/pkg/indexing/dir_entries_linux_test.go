//go:build linux

package indexing

import (
	"fmt"
	"os"
	"path/filepath"
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
