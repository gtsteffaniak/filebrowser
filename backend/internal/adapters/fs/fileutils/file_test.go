package fileutils

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUnixModeToFileMode_setgid(t *testing.T) {
	m := unixModeToFileMode(0o2770)
	if got := m.Perm(); got != 0o770 {
		t.Fatalf("Perm() = %#o, want 0770", got)
	}
	if m&os.ModeSetgid == 0 {
		t.Fatal("expected ModeSetgid")
	}
	if m&os.ModeSetuid != 0 || m&os.ModeSticky != 0 {
		t.Fatalf("unexpected special bits: %#o", m)
	}
}

func TestCommonPrefix(t *testing.T) {
	testCases := map[string]struct {
		paths []string
		want  string
	}{
		"same lvl": {
			paths: []string{
				"/home/user/file1",
				"/home/user/file2",
			},
			want: "/home/user",
		},
		"sub folder": {
			paths: []string{
				"/home/user/folder",
				"/home/user/folder/file",
			},
			want: "/home/user/folder",
		},
		"relative path": {
			paths: []string{
				"/home/user/folder",
				"/home/user/folder/../folder2",
			},
			want: "/home/user",
		},
		"no common path": {
			paths: []string{
				"/home/user/folder",
				"/etc/file",
			},
			want: "",
		},
	}
	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			if got := CommonPrefix('/', tt.paths...); got != tt.want {
				t.Errorf("CommonPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCopyFilePreservesModTime(t *testing.T) {
	fileTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	dirTime := time.Date(2026, 6, 7, 8, 9, 0, 0, time.UTC)
	testCases := map[string]struct {
		isDir     bool
		checkPath map[string]time.Time
	}{
		"file":      {checkPath: map[string]time.Time{"": fileTime}},
		"directory": {isDir: true, checkPath: map[string]time.Time{"": dirTime, "nested.txt": fileTime}},
	}
	for name, tt := range testCases {
		t.Run(name, func(t *testing.T) {
			src := t.TempDir()
			dst := filepath.Join(t.TempDir(), "dst")
			srcPath := src
			if tt.isDir {
				nested := filepath.Join(src, "nested.txt")
				if err := os.WriteFile(nested, []byte("hi"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(nested, fileTime, fileTime); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(src, dirTime, dirTime); err != nil {
					t.Fatal(err)
				}
			} else {
				srcPath = filepath.Join(src, "file.txt")
				if err := os.WriteFile(srcPath, []byte("hi"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Chtimes(srcPath, fileTime, fileTime); err != nil {
					t.Fatal(err)
				}
			}
			if err := CopyFile(srcPath, dst); err != nil {
				t.Fatal(err)
			}
			for rel, want := range tt.checkPath {
				p := dst
				if rel != "" {
					p = filepath.Join(dst, rel)
				}
				got, err := os.Stat(p)
				if err != nil {
					t.Fatal(err)
				}
				if !got.ModTime().Equal(want) {
					t.Errorf("%s: got mtime = %v, want %v", p, got.ModTime(), want)
				}
			}
		})
	}
}
