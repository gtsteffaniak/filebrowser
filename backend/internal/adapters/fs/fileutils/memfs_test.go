package fileutils

import (
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMemFSServesFromMemory(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.html":        "<html>index</html>",
		"assets/app.js.gz":  "gzip-bytes",
		"fonts/icons.woff2": "font-bytes",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	m, err := loadMemFS(dirFS{Dir: http.Dir(dir)})
	if err != nil {
		t.Fatalf("loadMemFS: %v", err)
	}
	if m.Len() != len(files) {
		t.Fatalf("loaded %d files, want %d", m.Len(), len(files))
	}

	// Remove everything from disk: the cache must still serve every file.
	if removeErr := os.RemoveAll(dir); removeErr != nil {
		t.Fatal(removeErr)
	}
	for name, want := range files {
		got, readErr := fs.ReadFile(m, name)
		if readErr != nil {
			t.Fatalf("ReadFile(%s) after disk removal: %v", name, readErr)
		}
		if string(got) != want {
			t.Errorf("ReadFile(%s) = %q, want %q", name, got, want)
		}
		info, statErr := fs.Stat(m, name)
		if statErr != nil {
			t.Fatalf("Stat(%s): %v", name, statErr)
		}
		if info.IsDir() || info.Size() != int64(len(want)) || info.Name() != filepath.Base(name) {
			t.Errorf("Stat(%s) = %+v, unexpected", name, info)
		}
	}

	// Template-style glob on a plain name must resolve via Stat.
	matches, globErr := fs.Glob(m, "index.html")
	if globErr != nil || len(matches) != 1 {
		t.Fatalf("Glob(index.html) = %v, %v", matches, globErr)
	}

	if _, readErr := fs.ReadFile(m, "missing.css"); readErr == nil {
		t.Error("ReadFile(missing.css) should fail")
	}
	if _, readErr := fs.ReadFile(m, "../escape"); readErr == nil {
		t.Error("invalid path should be rejected")
	}
}
