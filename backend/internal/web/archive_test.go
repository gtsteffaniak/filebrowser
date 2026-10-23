package web

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestTarGz(t *testing.T, path string, write func(tw *tar.Writer) error) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := write(tw); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

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
	if err := extractTarGz(archivePath, destDir); err != nil {
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
	if err := extractTarGz(missing, t.TempDir()); err == nil {
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
	if err := extractTarGz(replaced, replacedDest); err == nil {
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
	if err := extractTarGz(self, selfDest); err != nil {
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
		if err := extractTarGz(archivePath, destDir); err != nil {
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
		if err := extractTarGz(archivePath, destDir); err != nil {
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
		if err := extractTarGz(archivePath, destDir); err != nil {
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
	f, err := os.Create(zipPath)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("out.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = w.Write([]byte("payload")); err != nil {
		t.Fatal(err)
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if err = extractZip(zipPath, destDir); err != nil {
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

// Benchmarks (optional, keep small)
func BenchmarkNormalizeArchiveEntryName(b *testing.B) {
	const name = `PRO1000\Winx64\NDIS65\e1c65x64.cat`
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = normalizeArchiveEntryName(name)
	}
}
