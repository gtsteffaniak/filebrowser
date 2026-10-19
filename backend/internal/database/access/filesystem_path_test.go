package access

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	t.Cleanup(func() { fsOps = orig })
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

func TestResolveFilesystemIndexPathReResolvesAfterSegmentAppears(t *testing.T) {
	root := filepath.FromSlash("/data")
	dir := filepath.Join(root, "privatestorage")
	secret := filepath.Join(dir, "Secret.TXT")
	fileID := 99
	fileInfo := stubInfo{name: "Secret.TXT", mode: 0, id: fileID}
	created := false

	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			if name == dir {
				return stubInfo{name: "privatestorage", mode: fs.ModeDir, id: 1}, nil
			}
			if name == secret || name == filepath.Join(dir, "secret.txt") {
				if !created {
					return nil, fs.ErrNotExist
				}
				return fileInfo, nil
			}
			return nil, fs.ErrNotExist
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			if name == root {
				return []fs.DirEntry{mockDirEntry{info: stubInfo{name: "privatestorage", mode: fs.ModeDir, id: 1}}}, nil
			}
			if name == dir {
				if !created {
					return []fs.DirEntry{}, nil
				}
				return []fs.DirEntry{mockDirEntry{info: fileInfo}}, nil
			}
			return nil, fs.ErrNotExist
		},
	})

	got, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage/secret.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/secret.txt" {
		t.Fatalf("first resolve %q", got.String())
	}

	created = true
	got, err = resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/privatestorage/secret.txt", false))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "/privatestorage/Secret.TXT" {
		t.Fatalf("second resolve %q, want on-disk spelling", got.String())
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
	root := filepath.FromSlash("/data")
	dirID := 1
	fileID := 2
	privateDir := stubInfo{name: "privatestorage", mode: fs.ModeDir, id: dirID}
	secretFile := stubInfo{name: "secret.txt", mode: 0, id: fileID}
	privPath := filepath.Join(root, "privatestorage")
	aliasPath := filepath.Join(root, "PrivateStorage")
	secretPath := filepath.Join(privPath, "secret.txt")

	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			switch name {
			case aliasPath, privPath:
				return privateDir, nil
			case secretPath:
				return secretFile, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			switch name {
			case root:
				return []fs.DirEntry{mockDirEntry{info: privateDir}}, nil
			case privPath:
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

func TestResolveFilesystemIndexPathDeniesAmbiguousHardLink(t *testing.T) {
	root := filepath.FromSlash("/data")
	fileID := 42
	candidate := stubInfo{name: "PRIVATE.TXT", mode: 0, id: fileID}
	aFile := stubInfo{name: "a.txt", mode: 0, id: fileID}
	zFile := stubInfo{name: "z.txt", mode: 0, id: fileID}
	dir := stubInfo{name: "share", mode: fs.ModeDir, id: 1}
	share := filepath.Join(root, "share")
	candPath := filepath.Join(share, "PRIVATE.TXT")
	aPath := filepath.Join(share, "a.txt")
	zPath := filepath.Join(share, "z.txt")

	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			switch name {
			case share:
				return dir, nil
			case candPath, aPath, zPath:
				return candidate, nil
			default:
				return nil, fs.ErrNotExist
			}
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			switch name {
			case root:
				return []fs.DirEntry{mockDirEntry{info: dir}}, nil
			case share:
				return []fs.DirEntry{
					mockDirEntry{info: aFile},
					mockDirEntry{info: zFile},
				}, nil
			}
			return nil, fs.ErrNotExist
		},
		same: func(a, b fs.FileInfo) bool {
			ai, okA := a.(stubInfo)
			bi, okB := b.(stubInfo)
			return okA && okB && ai.id == bi.id
		},
	})

	_, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/share/PRIVATE.TXT", false))
	if err == nil {
		t.Fatal("expected ambiguous hard link resolution to fail closed")
	}
	if !strings.Contains(err.Error(), "ambiguous filesystem alias") {
		t.Fatalf("expected hard-link ambiguity error, got %v", err)
	}
}

func TestResolveFilesystemIndexPathReadDirEACCESFailsClosed(t *testing.T) {
	root := filepath.FromSlash("/data")
	aliasPath := filepath.Join(root, "PrivateStorage")
	withMockFS(t, &mockFS{
		lstat: func(name string) (fs.FileInfo, error) {
			if name == aliasPath {
				return stubInfo{name: "PrivateStorage", mode: fs.ModeDir, id: 1}, nil
			}
			return nil, fs.ErrNotExist
		},
		readdir: func(name string) ([]fs.DirEntry, error) {
			if name == root {
				return nil, syscall.EACCES
			}
			return nil, fs.ErrNotExist
		},
	})

	_, err := resolveFilesystemIndexPath(root, utils.IndexPathFromNormalized("/PrivateStorage/secret.txt", false))
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
