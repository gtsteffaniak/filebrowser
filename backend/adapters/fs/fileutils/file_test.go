package fileutils

import (
	"os"
	"testing"
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
