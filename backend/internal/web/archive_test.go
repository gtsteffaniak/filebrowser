package web

import (
	"archive/tar"
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	commonerrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestNormalizeArchiveEntryName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "simple forward slashes",
			input: "DOCS/guide.pdf",
			want:  "DOCS/guide.pdf",
		},
		{
			name:  "Windows backslashes become logical path",
			input: `DOCS\QUICK\chs.gif`,
			want:  "DOCS/QUICK/chs.gif",
		},
		{
			name:  "Intel-style leading backslash stripped so path stays relative",
			input: `\PRO1000\Winx64\NDIS65\e1c65x64.cat`,
			want:  "PRO1000/Winx64/NDIS65/e1c65x64.cat",
		},
		{
			name:  "leading forward slashes stripped",
			input: "/PRO1000/Winx64/readme.txt",
			want:  "PRO1000/Winx64/readme.txt",
		},
		{
			name:  "directory entry with trailing slash",
			input: `PRO1000\Winx64\`,
			want:  "PRO1000/Winx64",
		},
		{
			name:  "mixed redundant segments cleaned",
			input: "a/./b/../c/file",
			want:  "a/c/file",
		},
		{
			name:    "empty",
			input:   "",
			wantErr: "empty path",
		},
		{
			name:    "whitespace only",
			input:   "   ",
			wantErr: "empty path",
		},
		{
			name:    "UNC rejected",
			input:   `//server/share/file`,
			wantErr: "UNC or invalid path",
		},
		{
			name:    "double slash prefix after backslash replace (UNC-style)",
			input:   `\\?\C:\temp\file`,
			wantErr: "UNC or invalid path",
		},
		{
			name:    "Windows drive absolute rejected",
			input:   `C:/Windows/system32`,
			wantErr: "absolute path in archive",
		},
		{
			name:    "lowercase drive rejected",
			input:   `c:/data/file.txt`,
			wantErr: "absolute path in archive",
		},
		{
			name:    "bare dot-dot",
			input:   "..",
			wantErr: "path traversal",
		},
		{
			name:    "prefixed dot-dot",
			input:   "../etc/passwd",
			wantErr: "path traversal",
		},
		{
			name:    "traversal after path clean",
			input:   "safe/../../outside",
			wantErr: "path traversal after clean",
		},
		{
			name:    "only dots and slashes",
			input:   "./",
			wantErr: "empty path after clean",
		},
		{
			name:    "path cleans to nothing",
			input:   "x/../.",
			wantErr: "empty path after clean",
		},
		{
			name:  "spaces preserved in file name",
			input: "My Folder/read me.txt",
			want:  "My Folder/read me.txt",
		},
		{
			name:  "deep relative tree",
			input: "a/b/c/d/e/f/g.bin",
			want:  "a/b/c/d/e/f/g.bin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := normalizeArchiveEntryName(tt.input)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil (result %q)", tt.wantErr, got)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error %q should contain %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("normalizeArchiveEntryName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestNormalizeArchiveEntryName_backslashInFileNameOnUnix documents that after normalization
// a single backslash in the raw name is never a path separator: it becomes a slash in the
// logical path (portable), so segments split correctly on every GOOS in safeExtractPath.
func TestNormalizeArchiveEntryName_unifiesBackslashes(t *testing.T) {
	t.Parallel()
	got, err := normalizeArchiveEntryName(`folder\file.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "folder/file.txt"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestSafeExtractPath(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	dest, err := filepath.Abs(dest)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("file under dest", func(t *testing.T) {
		t.Parallel()
		got, err := safeExtractPath(dest, `DOCS\Adapter_User_Guide.pdf`)
		if err != nil {
			t.Fatal(err)
		}
		want := filepath.Join(dest, "DOCS", "Adapter_User_Guide.pdf")
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("leading backslash in zip name stays under dest", func(t *testing.T) {
		t.Parallel()
		got, err := safeExtractPath(dest, `\sub\file.txt`)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(got, dest+string(filepath.Separator)) {
			t.Fatalf("result %q should be under dest %q", got, dest)
		}
		rel, err := filepath.Rel(dest, got)
		if err != nil {
			t.Fatal(err)
		}
		if rel != filepath.Join("sub", "file.txt") {
			t.Fatalf("Rel = %q, want sub%cfile.txt", rel, filepath.Separator)
		}
	})

	t.Run("rejects path that escapes dest", func(t *testing.T) {
		t.Parallel()
		_, err := safeExtractPath(dest, "../../../x")
		if err == nil {
			t.Fatal("expected error for parent segments in archive name")
		}
		if !strings.Contains(err.Error(), "invalid entry path") {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("rejects when normalize fails", func(t *testing.T) {
		t.Parallel()
		_, err := safeExtractPath(dest, "..")
		if err == nil {
			t.Fatal("expected error")
		}
		if !strings.Contains(err.Error(), "invalid entry path") {
			t.Fatalf("got %v", err)
		}
	})
}

// TestSafeExtractPath_rejectsParentTraversal ensures ".." in the archive name is rejected
// by normalizeArchiveEntryName, not written under dest.
func TestSafeExtractPath_rejectsParentTraversal(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	_, err := safeExtractPath(dest, `pkg/../../../secret`)
	if err == nil {
		t.Fatal("expected error for path traversal in archive name")
	}
}

// TestSafeExtractPath_destJoinDoesNotDropBase verifies that a previously problematic pattern
// (root-relative second segment) does not throw away dest after normalization.
func TestSafeExtractPath_destJoinDoesNotDropBase(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	out, err := safeExtractPath(dest, `a\b\c.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, dest+string(filepath.Separator)) {
		t.Fatalf("expected path under %q, got %q", dest, out)
	}
}

// TestExtractZip_nestedBackslashes extracts a Windows-style path in a zip (backslashes) into
// nested directories under dest. This matches many vendor driver zips.
func TestExtractZip_nestedBackslashes(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	zipPath := filepath.Join(tmp, "test.zip")
	out := filepath.Join(tmp, "out")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}

	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create(`DOCS\QUICK\readme.txt`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("hello")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}

	if err = extractZip(zipPath, out); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(out, "DOCS", "QUICK", "readme.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hello" {
		t.Fatalf("content = %q", b)
	}
}

// Benchmarks (optional, keep small)
func BenchmarkNormalizeArchiveEntryName(b *testing.B) {
	const name = `PRO1000\Winx64\NDIS65\e1c65x64.cat`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = normalizeArchiveEntryName(name)
	}
}

func TestExtractZipFilenameEncoding(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, raw, charset, want string
		invalid                  bool
	}{
		{name: "cp932 trail byte", raw: "\x83\x65\x83\x58\x83\x67/\x83\x5c.txt", charset: "cp932", want: "テスト/ソ.txt"},
		{name: "automatic cp932", raw: "\x83\x65\x83\x58\x83\x67/\x83\x5c.txt", want: "テスト/ソ.txt"},
		{name: "traversal after decoding", raw: "../\x83\x5c.txt", charset: "cp932", invalid: true},
		{name: "invalid encoding", raw: "file.txt", charset: "unknown", invalid: true},
		{name: "ambiguous", raw: "caf\x82.txt", invalid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			archivePath := filepath.Join(t.TempDir(), "test.zip")
			f, err := os.Create(archivePath)
			if err != nil {
				t.Fatal(err)
			}
			zw := zip.NewWriter(f)
			w, err := zw.CreateHeader(&zip.FileHeader{Name: tc.raw, NonUTF8: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = w.Write([]byte("test content")); err != nil {
				t.Fatal(err)
			}
			if err = zw.Close(); err != nil {
				t.Fatal(err)
			}
			if err = f.Close(); err != nil {
				t.Fatal(err)
			}
			out := t.TempDir()
			err = extractZipWithEncoding(archivePath, out, tc.charset, nil)
			if tc.invalid {
				if err == nil {
					t.Fatal("expected error")
				}
				entries, readErr := os.ReadDir(out)
				if readErr != nil || len(entries) != 0 {
					t.Fatal("failed validation wrote files")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			content, err := os.ReadFile(filepath.Join(out, tc.want))
			if err != nil || string(content) != "test content" {
				t.Fatalf("content=%q err=%v", content, err)
			}
		})
	}
}

func TestExtractTarGz_hardLinks(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "links.tar.gz")
	writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
		if err := tw.WriteHeader(&tar.Header{Name: "proj/a.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, "same"); err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{Name: "proj/sub/b.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "proj/a.txt"})
	})
	destDir := t.TempDir()
	if err := extractTarGz(archivePath, destDir, nil); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(destDir, "proj", "sub", "b.txt"))
	if err != nil || string(got) != "same" {
		t.Fatalf("hard link entry: got %q, %v", got, err)
	}

	missing := filepath.Join(t.TempDir(), "missing.tar.gz")
	writeTestTarGz(t, missing, func(tw *tar.Writer) error {
		return tw.WriteHeader(&tar.Header{Name: "b.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "a.txt"})
	})
	if err := extractTarGz(missing, t.TempDir(), nil); err == nil {
		t.Fatal("expected an error for a hard link to a file that is not in the archive")
	}

	// A symlink entry replaces the extracted file before the hard link names it.
	replaced := filepath.Join(t.TempDir(), "replaced.tar.gz")
	writeTestTarGz(t, replaced, func(tw *tar.Writer) error {
		if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, "mine"); err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "private/secret.txt"}); err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{Name: "b.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "a.txt"})
	})
	replacedDest := t.TempDir()
	if err := os.MkdirAll(filepath.Join(replacedDest, "private"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(replacedDest, "private", "secret.txt"), []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractTarGz(replaced, replacedDest, nil); err == nil {
		t.Fatal("expected an error for a hard link whose target was replaced by a symlink")
	}
	if got, err := os.ReadFile(filepath.Join(replacedDest, "b.txt")); err == nil {
		t.Fatalf("hard link entry copied the symlink target: %q", got)
	}

	// A hard link names its own target, as GNU tar writes for a file archived twice,
	// or names it through a symlink.
	self := filepath.Join(t.TempDir(), "self.tar.gz")
	writeTestTarGz(t, self, func(tw *tar.Writer) error {
		if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
			return err
		}
		if _, err := io.WriteString(tw, "same"); err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: "c.txt", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "a.txt"}); err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "a.txt"}); err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{Name: "c.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "a.txt"})
	})
	selfDest := t.TempDir()
	if err := extractTarGz(self, selfDest, nil); err != nil {
		t.Fatalf("extract: %v", err)
	}
	if got, err := os.ReadFile(filepath.Join(selfDest, "a.txt")); err != nil || string(got) != "same" {
		t.Fatalf("hard link to itself: got %q, %v", got, err)
	}
}

func TestExtractTarGz_symlinkDestination(t *testing.T) {
	t.Run("pre-existing symlink at destination", func(t *testing.T) {
		destDir := t.TempDir()
		victim := filepath.Join(destDir, "victim.txt")
		if err := os.WriteFile(victim, []byte("untouched"), 0o644); err != nil {
			t.Fatal(err)
		}
		outPath := filepath.Join(destDir, "out.txt")
		if err := os.Symlink("victim.txt", outPath); err != nil {
			t.Fatal(err)
		}
		archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			if err := tw.WriteHeader(&tar.Header{Name: "out.txt", Mode: 0o644, Size: 7, Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			_, err := io.WriteString(tw, "payload")
			return err
		})
		if err := extractTarGz(archivePath, destDir, nil); err != nil {
			t.Fatalf("extract: %v", err)
		}
		got, err := os.ReadFile(outPath)
		if err != nil || string(got) != "payload" {
			t.Fatalf("out.txt: got %q, %v", got, err)
		}
		victimData, err := os.ReadFile(victim)
		if err != nil || string(victimData) != "untouched" {
			t.Fatalf("victim.txt should be unchanged: %q, %v", victimData, err)
		}
		info, err := os.Lstat(outPath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("out.txt should be a regular file, mode=%v err=%v", info.Mode(), err)
		}
	})

	t.Run("symlink entry then regular file at same path", func(t *testing.T) {
		destDir := t.TempDir()
		other := filepath.Join(destDir, "other.txt")
		if err := os.WriteFile(other, []byte("keep"), 0o644); err != nil {
			t.Fatal(err)
		}
		archivePath := filepath.Join(t.TempDir(), "link_then_reg.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			if err := tw.WriteHeader(&tar.Header{Name: "link", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "other.txt"}); err != nil {
				return err
			}
			if err := tw.WriteHeader(&tar.Header{Name: "link", Mode: 0o644, Size: 7, Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			_, err := io.WriteString(tw, "written")
			return err
		})
		if err := extractTarGz(archivePath, destDir, nil); err != nil {
			t.Fatalf("extract: %v", err)
		}
		linkPath := filepath.Join(destDir, "link")
		got, err := os.ReadFile(linkPath)
		if err != nil || string(got) != "written" {
			t.Fatalf("link: got %q, %v", got, err)
		}
		otherData, err := os.ReadFile(other)
		if err != nil || string(otherData) != "keep" {
			t.Fatalf("other.txt should be unchanged: %q, %v", otherData, err)
		}
	})

	t.Run("hard link onto symlink to source", func(t *testing.T) {
		destDir := t.TempDir()
		archivePath := filepath.Join(t.TempDir(), "hardlink_symlink_dest.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			if err := tw.WriteHeader(&tar.Header{Name: "a.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			if _, err := io.WriteString(tw, "same"); err != nil {
				return err
			}
			if err := tw.WriteHeader(&tar.Header{Name: "b.txt", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "a.txt"}); err != nil {
				return err
			}
			return tw.WriteHeader(&tar.Header{Name: "c.txt", Mode: 0o644, Typeflag: tar.TypeLink, Linkname: "a.txt"})
		})
		if err := extractTarGz(archivePath, destDir, nil); err != nil {
			t.Fatalf("extract: %v", err)
		}
		cPath := filepath.Join(destDir, "c.txt")
		got, err := os.ReadFile(cPath)
		if err != nil || string(got) != "same" {
			t.Fatalf("c.txt: got %q, %v", got, err)
		}
		info, err := os.Lstat(cPath)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			t.Fatalf("c.txt should be a regular file")
		}
	})
}

func TestExtractZip_symlinkDestination(t *testing.T) {
	destDir := t.TempDir()
	victim := filepath.Join(destDir, "victim.txt")
	if err := os.WriteFile(victim, []byte("untouched"), 0o644); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(destDir, "out.txt")
	if err := os.Symlink("victim.txt", outPath); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "out.zip")
	writeTestZip(t, zipPath, func(zw *zip.Writer) error {
		w, err := zw.Create("out.txt")
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("payload"))
		return err
	})
	if err := extractZip(zipPath, destDir); err != nil {
		t.Fatalf("extract: %v", err)
	}
	got, err := os.ReadFile(outPath)
	if err != nil || string(got) != "payload" {
		t.Fatalf("out.txt: got %q, %v", got, err)
	}
	victimData, err := os.ReadFile(victim)
	if err != nil || string(victimData) != "untouched" {
		t.Fatalf("victim.txt should be unchanged: %q, %v", victimData, err)
	}
}

func writeChainedSymlinkEscapeTar(t *testing.T, path string) {
	t.Helper()
	writeTestTarGz(t, path, func(tw *tar.Writer) error {
		if err := tw.WriteHeader(&tar.Header{Name: "a/b/", Mode: 0o755, Typeflag: tar.TypeDir}); err != nil {
			return err
		}
		if err := tw.WriteHeader(&tar.Header{Name: "a/b/l1", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "../.."}); err != nil {
			return err
		}
		return tw.WriteHeader(&tar.Header{Name: "esc", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "a/b/l1/../x"})
	})
}

func writeChainedSymlinkEscapeZip(t *testing.T, path string) {
	t.Helper()
	writeTestZip(t, path, func(zw *zip.Writer) error {
		hdr := &zip.FileHeader{Name: "a/b/l1", Method: zip.Store}
		hdr.SetMode(0o777 | os.ModeSymlink)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if _, err = w.Write([]byte("../..")); err != nil {
			return err
		}
		hdr = &zip.FileHeader{Name: "esc", Method: zip.Store}
		hdr.SetMode(0o777 | os.ModeSymlink)
		w, err = zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("a/b/l1/../x"))
		return err
	})
}

func assertNoEscapingSymlinksUnderDest(t *testing.T, destDir string) {
	t.Helper()
	destResolved, err := filepath.EvalSymlinks(destDir)
	if err != nil {
		t.Fatal(err)
	}
	sep := string(filepath.Separator)
	walkErr := filepath.WalkDir(destDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}
		target, err := os.Readlink(path)
		if err != nil {
			return err
		}
		linkDir, err := resolvedSymlinkParentDir(path)
		if err != nil {
			return err
		}
		expansionsLeft := maxSymlinkResolveExpansions
		resolved, err := resolveSymlinkTargetAt(linkDir, target, 0, &expansionsLeft)
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(destResolved, resolved)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+sep) {
			t.Fatalf("symlink %q resolves outside destination to %q", path, resolved)
		}
		return nil
	})
	if walkErr != nil {
		t.Fatal(walkErr)
	}
}

func TestSymlinkTargetStaysUnderDest_rejectsChainedRelativeEscape(t *testing.T) {
	destDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(destDir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	l1 := filepath.Join(destDir, "a", "b", "l1")
	if err := os.Symlink("../..", l1); err != nil {
		t.Fatal(err)
	}
	escPath := filepath.Join(destDir, "esc")
	if err := symlinkTargetStaysUnderDest(destDir, escPath, "a/b/l1/../x"); err == nil {
		t.Fatal("expected chained symlink target to be rejected")
	}
}

func TestExtractZip_rejectsChainedSymlinkEscape(t *testing.T) {
	destDir := t.TempDir()
	zipPath := filepath.Join(t.TempDir(), "chain.zip")
	writeChainedSymlinkEscapeZip(t, zipPath)
	err := extractZip(zipPath, destDir)
	if err == nil || !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected rejection, got %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(destDir, "esc")); !os.IsNotExist(statErr) {
		t.Fatal("escaping symlink should not be created")
	}
	assertNoEscapingSymlinksUnderDest(t, destDir)
}

func TestExtractTarGz_rejectsChainedSymlinkEscape(t *testing.T) {
	destDir := t.TempDir()
	archivePath := filepath.Join(t.TempDir(), "chain.tar.gz")
	writeChainedSymlinkEscapeTar(t, archivePath)
	err := extractTarGz(archivePath, destDir, nil)
	if err == nil || !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected rejection, got %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(destDir, "esc")); !os.IsNotExist(statErr) {
		t.Fatal("escaping symlink should not be created")
	}
	assertNoEscapingSymlinksUnderDest(t, destDir)
}

func TestExtractZip_rejectsChainedSymlinkEscape_preExistingHop(t *testing.T) {
	destDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(destDir, "a", "b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../..", filepath.Join(destDir, "a", "b", "l1")); err != nil {
		t.Fatal(err)
	}
	zipPath := filepath.Join(t.TempDir(), "hop.zip")
	writeTestZip(t, zipPath, func(zw *zip.Writer) error {
		hdr := &zip.FileHeader{Name: "esc", Method: zip.Store}
		hdr.SetMode(0o777 | os.ModeSymlink)
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("a/b/l1/../x"))
		return err
	})
	err := extractZip(zipPath, destDir)
	if err == nil || !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected rejection, got %v", err)
	}
	if _, statErr := os.Lstat(filepath.Join(destDir, "esc")); !os.IsNotExist(statErr) {
		t.Fatal("escaping symlink should not be created")
	}
	assertNoEscapingSymlinksUnderDest(t, destDir)
}

func TestResolveSymlinkTargetAt_rejectsSymlinkExpansionBomb(t *testing.T) {
	dest := t.TempDir()
	if err := os.Symlink(".", filepath.Join(dest, "l0")); err != nil {
		t.Fatal(err)
	}
	prev := "l0"
	for i := 1; i <= 25; i++ {
		name := fmt.Sprintf("l%d", i)
		target := prev + "/" + prev
		if err := os.Symlink(target, filepath.Join(dest, name)); err != nil {
			t.Fatal(err)
		}
		prev = name
	}
	expansionsLeft := maxSymlinkResolveExpansions
	start := time.Now()
	_, err := resolveSymlinkTargetAt(dest, "l25/l25", 0, &expansionsLeft)
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "escapes destination") {
		t.Fatalf("expected rejection, got %v after %v", err, elapsed)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("symlink expansion bomb took too long: %v", elapsed)
	}
}

func TestSymlinkTargetStaysUnderDest_rejectsAbsolutePreExistingHop(t *testing.T) {
	destDir := t.TempDir()
	outside := filepath.Join(filepath.Dir(destDir), "outside-abs-hop")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(destDir, "bad")); err != nil {
		t.Fatal(err)
	}
	newLink := filepath.Join(destDir, "newlink")
	if err := symlinkTargetStaysUnderDest(destDir, newLink, "bad"); err == nil {
		t.Fatal("expected absolute pre-existing hop outside dest to be rejected")
	}
}

func TestSymlinkTargetStaysUnderDest_hopTargetVerbatimReadlink(t *testing.T) {
	destDir := t.TempDir()
	allowed := filepath.Join(destDir, "allowed.txt")
	if err := os.WriteFile(allowed, []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	hop := filepath.Join(destDir, "hop")
	// Trailing space must be preserved (no TrimSpace on Readlink); the literal name is not allowed.txt.
	if err := os.Symlink("allowed.txt ", hop); err != nil {
		t.Fatal(err)
	}
	got, err := os.Readlink(hop)
	if err != nil || got != "allowed.txt " {
		t.Fatalf("readlink verbatim target: %q err=%v", got, err)
	}
	expansionsLeft := maxSymlinkResolveExpansions
	resolved, err := resolveSymlinkTargetAt(destDir, "hop", 0, &expansionsLeft)
	if err != nil {
		t.Fatalf("resolve hop: %v", err)
	}
	want := filepath.Join(destDir, "allowed.txt ")
	if resolved != want {
		t.Fatalf("hop should resolve to literal spaced name %q, got %q", want, resolved)
	}
	if _, statErr := os.Stat(resolved); !os.IsNotExist(statErr) {
		t.Fatalf("spaced target should not alias allowed.txt without trimming: stat %v", statErr)
	}
	if _, statErr := os.Stat(allowed); statErr != nil {
		t.Fatalf("allowed.txt should exist: %v", statErr)
	}
}

func TestArchiveCreateDestination_RejectsEscapingSymlinkLeaf(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	outside := filepath.Join(filepath.Dir(root), "outside-archive")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "report.zip")); err != nil {
		t.Fatal(err)
	}
	idx := &indexing.Index{Source: settings.Source{Path: root}}
	if _, err := idx.GetRealPathForMutation("", "/report.zip"); !errors.Is(err, commonerrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope, got %v", err)
	}
}
