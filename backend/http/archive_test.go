package http

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
