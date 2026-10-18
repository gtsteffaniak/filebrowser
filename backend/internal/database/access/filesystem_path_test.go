package access

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

type stubInfo struct {
	name string
	mode fs.FileMode
	id   int
}

func (s stubInfo) Name() string       { return s.name }
func (s stubInfo) Size() int64        { return 0 }
func (s stubInfo) Mode() fs.FileMode { return s.mode }
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
	lstat   func(name string) (fs.FileInfo, error)
	readdir func(name string) ([]fs.DirEntry, error)
	same    func(a, b fs.FileInfo) bool
}

func (m *mockFS) Lstat(name string) (fs.FileInfo, error) {
	if m.lstat != nil {
		return m.lstat(name)
	}
	return nil, fs.ErrNotExist
}

func (m *mockFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if m.readdir != nil {
		return m.readdir(name)
	}
	return nil, fs.ErrNotExist
}

func (m *mockFS) SameFile(a, b fs.FileInfo) bool {
	if m.same != nil {
		return m.same(a, b)
	}
	ai, okA := a.(stubInfo)
	bi, okB := b.(stubInfo)
	return okA && okB && ai.id == bi.id
}

func withMockFS(t *testing.T, m *mockFS) {
	t.Helper()
	orig := fsOps
	fsOps = m
	filesystemPathCache.ClearAll()
	t.Cleanup(func() {
		fsOps = orig
		filesystemPathCache.ClearAll()
	})
}

func TestResolveFilesystemIndexPathFollowsExistingSegment(t *testing.T) {
	root := t.TempDir()
	privateDir := filepath.Join(root, "privatestorage")
	if err := os.MkdirAll(privateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(privateDir, "secret.txt")
	if err := os.WriteFile(secret, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage/secret.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/secret.txt" {
		t.Fatalf("resolved %q", got.String())
	}
}

func TestResolveFilesystemIndexPathPreservesNotExistTail(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "privatestorage"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage/newfile.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/newfile.txt" {
		t.Fatalf("resolved %q", got.String())
	}
}

func TestResolveFilesystemIndexPathPreservesIsDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "privatestorage"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage", true))
	if err != nil {
		t.Fatal(err)
	}
	if !got.IsDir {
		t.Fatal("expected directory semantics preserved")
	}
	if got.String() != "/privatestorage/" {
		t.Fatalf("resolved %q", got.String())
	}
}

func TestResolveFilesystemIndexPathCaseAliasViaReadDir(t *testing.T) {
	root := "/data"
	dirID := 1
	fileID := 2
	privateDir := stubInfo{name: "privatestorage", mode: fs.ModeDir, id: dirID}
	secretFile := stubInfo{name: "secret.txt", mode: 0, id: fileID}
	aliasDir := stubInfo{name: "PrivateStorage", mode: fs.ModeDir, id: dirID}

	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			switch name {
			case "/data/PrivateStorage":
				return aliasDir, nil
			case "/data/privatestorage/secret.txt":
				return secretFile, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			switch name {
			case "/data":
				return []fs.DirEntry{mockDirEntry{info: privateDir}}, nil
			case "/data/privatestorage":
				return []fs.DirEntry{mockDirEntry{info: secretFile}}, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
	})

	got, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/PrivateStorage/secret.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/secret.txt" {
		t.Fatalf("resolved %q, want /privatestorage/secret.txt", got.String())
	}
}

func TestResolveFilesystemIndexPathReadDirEACCESFailsClosed(t *testing.T) {
	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			if name == "/data/PrivateStorage" {
				return stubInfo{name: "PrivateStorage", mode: fs.ModeDir, id: 1}, nil
			}
			return nil, fs.ErrNotExist
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			if name == "/data" {
				return nil, syscall.EACCES
			}
			return nil, fs.ErrNotExist
		},
	})

	_, err := resolveFilesystemIndexPath("/data", utils.IndexPathFromNormalized("/PrivateStorage/secret.txt", false))
	if err == nil {
		t.Fatal("expected error on EACCES")
	}
	if !errors.Is(err, syscall.EACCES) {
		t.Fatalf("expected EACCES, got %v", err)
	}
}

func TestPermittedFailsClosedOnFilesystemResolveError(t *testing.T) {
	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			return nil, syscall.EACCES
		},
	})
	store := &Storage{}
	if store.Permitted("/data", utils.IndexPathFromNormalized("/any/path.txt", false), "alice") {
		t.Fatal("expected deny when filesystem resolution fails")
	}
}
