package indexing

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	liberrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
)

func TestToExtendedLengthPath(t *testing.T) {
	longTail := strings.Repeat("a", 260)
	longSlashTail := strings.Repeat("sub/", 70) + "file"
	longBackslashTail := strings.Repeat(`sub\`, 70) + "file"

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "short drive path unchanged",
			in:   `C:\src\file.txt`,
			want: `C:\src\file.txt`,
		},
		{
			name: "short UNC unchanged",
			in:   `\\server\share\file.txt`,
			want: `\\server\share\file.txt`,
		},
		{
			name: "already extended-length prefixed unchanged",
			in:   `\\?\C:\` + longTail,
			want: `\\?\C:\` + longTail,
		},
		{
			name: "already device-prefixed unchanged",
			in:   `\\.\C:\` + longTail,
			want: `\\.\C:\` + longTail,
		},
		{
			name: "long drive path gets extended-length prefix",
			in:   `C:\` + longTail,
			want: `\\?\C:\` + longTail,
		},
		{
			name: "long drive path with forward slashes is prefixed and normalized",
			in:   "C:/" + longSlashTail,
			want: `\\?\C:\` + longBackslashTail,
		},
		{
			name: "long UNC path becomes UNC extended-length path",
			in:   `\\server\share\` + longTail,
			want: `\\?\UNC\server\share\` + longTail,
		},
		{
			name: "long relative path unchanged",
			in:   `relative\` + longTail,
			want: `relative\` + longTail,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := toExtendedLengthPath(tc.in); got != tc.want {
				t.Fatalf("toExtendedLengthPath(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestStripFinalPathPrefix(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "dos drive prefix",
			in:   `\\?\C:\src\a`,
			want: `C:\src\a`,
		},
		{
			name: "unc prefix",
			in:   `\\?\UNC\srv\share\a`,
			want: `\\srv\share\a`,
		},
		{
			name: "plain path unchanged",
			in:   `C:\plain`,
			want: `C:\plain`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripFinalPathPrefix(tc.in); got != tc.want {
				t.Fatalf("stripFinalPathPrefix(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestTargetWithinRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")

	tests := []struct {
		name   string
		target string
		want   bool
	}{
		{
			name:   "root itself",
			target: root,
			want:   true,
		},
		{
			name:   "nested child",
			target: filepath.Join(root, "child", "x"),
			want:   true,
		},
		{
			name:   "sibling directory",
			target: filepath.Join(base, "sibling"),
			want:   false,
		},
		{
			name:   "prefix trick",
			target: root + "evil",
			want:   false,
		},
		{
			name:   "parent of root",
			target: base,
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := targetWithinRoot(root, tc.target); got != tc.want {
				t.Fatalf("targetWithinRoot(%q, %q) = %v, want %v", root, tc.target, got, tc.want)
			}
		})
	}
}

func TestResolveSymlinksContainedRejectsOutOfRootLink(t *testing.T) {
	base := t.TempDir()
	sourceRoot := filepath.Join(base, "source")
	insideSubdir := filepath.Join(sourceRoot, "subdir")
	outsideDir := filepath.Join(base, "outside")

	if err := os.MkdirAll(insideSubdir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outsideDir, 0o755); err != nil {
		t.Fatal(err)
	}

	insideLink := filepath.Join(sourceRoot, "inside_link")
	if err := os.Symlink(insideSubdir, insideLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	outsideLink := filepath.Join(sourceRoot, "outside_link")
	if err := os.Symlink(outsideDir, outsideLink); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	idx := symlinkTestIndex(sourceRoot)

	if _, err := idx.resolveSymlinksContained(outsideLink, "/"); !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope for out-of-root link, got %v", err)
	}

	got, err := idx.resolveSymlinksContained(insideLink, "/")
	if err != nil {
		t.Fatalf("expected in-root link to resolve, got error: %v", err)
	}
	want, err := resolveLinkTarget(insideSubdir)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("resolveSymlinksContained(%q) = %q, want %q", insideLink, got, want)
	}
}
