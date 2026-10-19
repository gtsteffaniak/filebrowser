package access

import (
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type stubInfo struct {
	name string
	mode fs.FileMode
}

func (s stubInfo) Name() string       { return s.name }
func (s stubInfo) Size() int64        { return 0 }
func (s stubInfo) Mode() fs.FileMode  { return s.mode }
func (s stubInfo) IsDir() bool        { return s.mode.IsDir() }
func (s stubInfo) ModTime() time.Time { return time.Time{} }
func (s stubInfo) Sys() interface{}   { return nil }

type stubDirEntry struct {
	name string
}

func (e stubDirEntry) Name() string               { return e.name }
func (e stubDirEntry) IsDir() bool                { return true }
func (e stubDirEntry) Type() fs.FileMode          { return fs.ModeDir }
func (e stubDirEntry) Info() (fs.FileInfo, error) { return stubInfo{name: e.name, mode: fs.ModeDir}, nil }

func withSourceFS(t *testing.T, fs *TestAccessFS) {
	t.Helper()
	SetSourceFSForTest(fs)
	t.Cleanup(func() { SetSourceFSForTest(nil) })
}

func setInsensitiveSource(s *Storage, sourcePath string) {
	s.mux.Lock()
	if s.sourceCaseInsensitive == nil {
		s.sourceCaseInsensitive = make(map[string]bool)
	}
	s.sourceCaseInsensitive[sourcePath] = true
	s.mux.Unlock()
}

func TestDetectCaseInsensitiveFromReadDir(t *testing.T) {
	root := "/data"
	withSourceFS(t, &TestAccessFS{
		ReadDirFn: func(name string) ([]fs.DirEntry, error) {
			if name == root {
				return []fs.DirEntry{stubDirEntry{name: "Photos"}}, nil
			}
			return nil, fs.ErrNotExist
		},
		LstatFn: func(name string) (fs.FileInfo, error) {
			if name == filepath.Join(root, "pHOTOS") {
				return stubInfo{name: "pHOTOS", mode: fs.ModeDir}, nil
			}
			return nil, fs.ErrNotExist
		},
	})
	if !DetectCaseInsensitiveSource(root) {
		t.Fatal("expected case-insensitive detection")
	}
}

func TestPermittedInsensitiveDenyCaseVariant(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: true}},
			},
		},
	}
	s := NewStorage(nil)
	setInsensitiveSource(s, "/data")
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/private/secret.txt", false), "alice"); err != nil {
		t.Fatal(err)
	}
	key := s.ruleKeyFor("/data", utils.IndexPathFromNormalized("/private/secret.txt", false))
	if key != "/private/secret.txt/" {
		t.Fatalf("stored key %q", key)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/PRIVATE/secret.txt", false), "alice") {
		t.Fatal("case variant must hit deny rule")
	}
}

func TestPermittedCaseSensitiveUnchanged(t *testing.T) {
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	s.mux.Lock()
	s.sourceCaseInsensitive = map[string]bool{"/data": false}
	s.mux.Unlock()
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/private/x", false), "alice"); err != nil {
		t.Fatal(err)
	}
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/PRIVATE/x", false), "alice") {
		t.Fatal("case-sensitive source must not apply /private deny to /PRIVATE path")
	}
}

func TestPermittedHotPathNoFilesystemAccess(t *testing.T) {
	withSourceFS(t, &TestAccessFS{
		ReadDirFn: func(string) ([]fs.DirEntry, error) {
			t.Fatal("unexpected filesystem access during Permitted")
			return nil, nil
		},
		LstatFn: func(string) (fs.FileInfo, error) {
			t.Fatal("unexpected filesystem access during Permitted")
			return nil, nil
		},
	})
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	setInsensitiveSource(s, "/data")
	_ = s.DenyUser("/data", utils.IndexPathFromNormalized("/a/b", false), "u")
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/a/b", false), "other") {
		t.Fatal("expected permitted for non-denied user")
	}
}

func BenchmarkPermittedInsensitiveNoFS(b *testing.B) {
	SetSourceFSForTest(&TestAccessFS{
		ReadDirFn: func(string) ([]fs.DirEntry, error) { panic("fs on hot path") },
		LstatFn:   func(string) (fs.FileInfo, error) { panic("fs on hot path") },
	})
	defer func() { SetSourceFSForTest(nil) }()

	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	s.mux.Lock()
	s.sourceCaseInsensitive = map[string]bool{"/data": true}
	s.mux.Unlock()
	_ = s.DenyUser("/data", utils.IndexPathFromNormalized("/x/y", false), "u")
	path := utils.IndexPathFromNormalized("/x/y", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Permitted("/data", path, "bob")
	}
}
