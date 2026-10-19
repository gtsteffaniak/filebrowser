package access

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

type stubInfo struct {
	name string
	mode fs.FileMode
	id   int
}

func (s stubInfo) Name() string       { return s.name }
func (s stubInfo) Size() int64        { return 0 }
func (s stubInfo) Mode() fs.FileMode  { return s.mode }
func (s stubInfo) ModTime() time.Time { return time.Time{} }
func (s stubInfo) IsDir() bool        { return s.mode.IsDir() }
func (s stubInfo) Sys() interface{}   { return s.id }

type mockDirEntry struct {
	info stubInfo
}

func (m mockDirEntry) Name() string               { return m.info.name }
func (m mockDirEntry) IsDir() bool                { return m.info.IsDir() }
func (m mockDirEntry) Type() fs.FileMode          { return m.info.Mode().Type() }
func (m mockDirEntry) Info() (fs.FileInfo, error) { return m.info, nil }

type mockFS struct {
	readdir func(name string) ([]fs.DirEntry, error)
}

func (m *mockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if m.readdir != nil {
		return m.readdir(name)
	}
	return nil, fs.ErrNotExist
}

func withMockFS(t *testing.T, m *mockFS) {
	t.Helper()
	orig := fsOps
	fsOps = m
	t.Cleanup(func() { fsOps = orig })
}

func testStorage(t *testing.T) *Storage {
	s := NewStorage(nil)
	s.markSourceCaseInsensitive("/data", true)
	return s
}

func (s *Storage) markSourceCaseInsensitive(sourcePath string, v bool) {
	s.mux.Lock()
	defer s.mux.Unlock()
	if s.caseInsensitive == nil {
		s.caseInsensitive = make(map[string]bool)
	}
	s.caseInsensitive[sourcePath] = v
}

func TestFoldedRuleIndexAddRemove(t *testing.T) {
	s := NewStorage(nil)
	path := "/secret/"
	s.mux.Lock()
	s.setFoldedRuleNL("src", path)
	s.mux.Unlock()
	s.mux.RLock()
	got := s.foldedRulePaths["src"][foldedRuleKey(path)]
	s.mux.RUnlock()
	if got != path {
		t.Fatalf("fold index %q", got)
	}
	s.mux.Lock()
	s.unsetFoldedRuleNL("src", path)
	s.mux.Unlock()
	s.mux.RLock()
	_, ok := s.foldedRulePaths["src"]
	s.mux.RUnlock()
	if ok {
		t.Fatal("expected folded map removed for source")
	}
}

func TestPermittedHotPathNoFilesystemCalls(t *testing.T) {
	var readDirCalls atomic.Int32
	withMockFS(t, &mockFS{
		readdir: func(name string) ([]fs.DirEntry, error) {
			readDirCalls.Add(1)
			return nil, errors.New("unexpected filesystem access")
		},
	})

	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}

	s := testStorage(t)
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/privatestorage/secret.txt", false), "alice"); err != nil {
		t.Fatal(err)
	}

	readDirCalls.Store(0)
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/privatestorage/secret.txt", false), "bob") {
		t.Fatal("expected permitted for non-denied user on exact path")
	}
	if readDirCalls.Load() != 0 {
		t.Fatalf("expected no ReadDir on exact-match hot path, got %d", readDirCalls.Load())
	}

	readDirCalls.Store(0)
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/other/none.txt", false), "bob") {
		t.Fatal("expected permitted when no rule applies and denyByDefault is false")
	}
	if readDirCalls.Load() != 0 {
		t.Fatalf("expected no ReadDir when no folded rule applies, got %d", readDirCalls.Load())
	}

	s.markSourceCaseInsensitive("/data", false)
	readDirCalls.Store(0)
	if !s.Permitted("/data", utils.IndexPathFromNormalized("/PRIVATE/secret.txt", false), "bob") {
		t.Fatal("case-sensitive source should not resolve")
	}
	if readDirCalls.Load() != 0 {
		t.Fatalf("expected no ReadDir on case-sensitive source, got %d", readDirCalls.Load())
	}
}

func TestPermittedCaseVariantDenyOnCaseInsensitiveSource(t *testing.T) {
	root := filepath.FromSlash("/data")
	dir := stubInfo{name: "privatestorage", mode: fs.ModeDir, id: 1}
	secret := stubInfo{name: "secret.txt", mode: 0, id: 2}
	privPath := filepath.Join(root, "privatestorage")
	aliasPath := filepath.Join(root, "PrivateStorage")
	secretPath := filepath.Join(privPath, "secret.txt")

	withMockFS(t, &mockFS{
		readdir: func(name string) ([]fs.DirEntry, error) {
			switch name {
			case root:
				return []fs.DirEntry{mockDirEntry{info: dir}}, nil
			case privPath:
				return []fs.DirEntry{mockDirEntry{info: secret}}, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
	})

	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				root: {Path: root, Config: settings.SourceConfig{DenyByDefault: true}},
			},
		},
	}

	s := testStorage(t)
	if err := s.DenyUser(root, utils.IndexPathFromNormalized("/privatestorage/secret.txt", false), "alice"); err != nil {
		t.Fatal(err)
	}
	if s.Permitted(root, utils.IndexPathFromNormalized("/PrivateStorage/secret.txt", false), "alice") {
		t.Fatal("case variant must match deny rule")
	}
	_ = aliasPath
	_ = secretPath
}

func TestResolveSegmentAmbiguityFailsClosed(t *testing.T) {
	root := filepath.FromSlash("/data/share")
	withMockFS(t, &mockFS{
		readdir: func(name string) ([]fs.DirEntry, error) {
			if name == root {
				return []fs.DirEntry{
					mockDirEntry{info: stubInfo{name: "file.txt", id: 1}},
					mockDirEntry{info: stubInfo{name: "File.txt", id: 2}},
				}, nil
			}
			return nil, fs.ErrNotExist
		},
	})
	_, err := resolveSegmentFromDir(root, "FILE.TXT")
	if err == nil {
		t.Fatal("expected ambiguity error")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("got %v", err)
	}
}

func TestPermittedFailsClosedOnFilesystemResolveError(t *testing.T) {
	withMockFS(t, &mockFS{
		readdir: func(name string) ([]fs.DirEntry, error) {
			return nil, syscall.EACCES
		},
	})
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: true}},
			},
		},
	}
	s := testStorage(t)
	if err := s.DenyUser("/data", utils.IndexPathFromNormalized("/privatestorage/secret.txt", false), "alice"); err != nil {
		t.Fatal(err)
	}
	if s.Permitted("/data", utils.IndexPathFromNormalized("/PrivateStorage/secret.txt", false), "alice") {
		t.Fatal("expected deny when filesystem resolution fails")
	}
}

func TestResolveIndexPathPreservesNotExistTail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "privatestorage"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveIndexPathForAccess(root, utils.IndexPathFromNormalized("/privatestorage/newfile.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/newfile.txt" {
		t.Fatalf("resolved %q", got.String())
	}
}

func BenchmarkPermittedExactPathNoFS(b *testing.B) {
	orig := fsOps
	fsOps = &mockFS{
		readdir: func(name string) ([]fs.DirEntry, error) {
			panic("filesystem used on hot path")
		},
	}
	defer func() { fsOps = orig }()
	settings.Config = settings.Settings{
		Server: settings.Server{
			SourceMap: map[string]*settings.Source{
				"/data": {Path: "/data", Config: settings.SourceConfig{DenyByDefault: false}},
			},
		},
	}
	s := NewStorage(nil)
	s.markSourceCaseInsensitive("/data", true)
	_ = s.DenyUser("/data", utils.IndexPathFromNormalized("/privatestorage/secret.txt", false), "alice")
	path := utils.IndexPathFromNormalized("/privatestorage/secret.txt", false)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if !s.Permitted("/data", path, "bob") {
			b.Fatal("expected permitted on exact-path hot path")
		}
	}
}
