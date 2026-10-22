package files

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"golang.org/x/net/webdav"
)

func mutationFixture(t *testing.T) (*indexing.Index, string) {
	t.Helper()
	fileutils.SetFsPermissions(0644, 0755)
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	indexing.SetTestIndex("mutation-guard", root)
	t.Cleanup(indexing.ClearTestIndices)
	idx := indexing.GetIndex("mutation-guard")
	idx.Config.ResolvedRules.IndexingDisabled = true
	return idx, root
}

func clientLock(t *testing.T, idx *indexing.Index, path string) string {
	t.Helper()
	token, err := idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: filepath.ToSlash(path), Duration: -1})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = idx.WebdavLock.Unlock(time.Now(), token) })
	return token
}

func TestMutationGuardBlockedOperations(t *testing.T) {
	for _, operation := range []string{"write", "delete", "delete-parent", "move", "move-parent", "move-destination", "copy-destination", "mkdir"} {
		t.Run(operation, func(t *testing.T) {
			idx, root := mutationFixture(t)
			dir := filepath.Join(root, "dir")
			if err := os.Mkdir(dir, 0755); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "file")
			if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
				t.Fatal(err)
			}
			clientLock(t, idx, path)
			var err error
			switch operation {
			case "write":
				err = writeFile(idx.Name, "/", "/dir/file", strings.NewReader("new"))
			case "delete":
				err = DeleteFiles(idx.Name, "/", "/dir/file", false)
			case "delete-parent":
				err = DeleteFiles(idx.Name, "/", "/dir", true)
			case "move":
				err = moveResourceImpl(false, idx.Name, idx.Name, path, filepath.Join(root, "moved"), nil)
			case "move-parent":
				err = moveResourceImpl(true, idx.Name, idx.Name, dir, filepath.Join(root, "moved"), nil)
			case "move-destination":
				src := filepath.Join(root, "source")
				if e := os.WriteFile(src, []byte("source"), 0644); e != nil {
					t.Fatal(e)
				}
				err = moveResourceImpl(false, idx.Name, idx.Name, src, path, nil)
			case "copy-destination":
				err = CopyResource(false, idx.Name, idx.Name, path, dir)
			case "mkdir":
				err = WriteDirectory("/", utils.FileOptions{Source: idx.Name, Path: "/dir/file"})
			}
			if !errors.Is(err, webdav.ErrLocked) {
				t.Fatalf("got %v, want ErrLocked", err)
			}
			content, e := os.ReadFile(path)
			if e != nil || string(content) != "original" {
				t.Fatalf("locked content changed: %q, %v", content, e)
			}
		})
	}
}

func TestMutationGuardLeaseLifetime(t *testing.T) {
	idx, root := mutationFixture(t)
	release, err := acquireMutation(mutationTarget{index: idx, path: filepath.Join(root, "dir")})
	if err != nil {
		t.Fatal(err)
	}
	_, err = idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: filepath.ToSlash(filepath.Join(root, "dir", "file")), Duration: -1})
	if !errors.Is(err, webdav.ErrLocked) {
		t.Fatalf("active mutation allowed client lock: %v", err)
	}
	release()
	token, err := idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: filepath.ToSlash(filepath.Join(root, "dir", "file")), Duration: -1})
	if err != nil {
		t.Fatalf("lease leaked: %v", err)
	}
	_ = idx.WebdavLock.Unlock(time.Now(), token)
}

func TestMutationGuardRejectsSymlinkAlias(t *testing.T) {
	idx, root := mutationFixture(t)
	real := filepath.Join(root, "real")
	if err := os.Mkdir(real, 0755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(real, alias); err != nil {
		t.Fatal(err)
	}
	_, err := acquireMutation(mutationTarget{index: idx, path: filepath.Join(alias, "file")})
	if !errors.Is(err, webdav.ErrLocked) {
		t.Fatalf("symlink alias permitted: %v", err)
	}
}

func TestMutationGuardNormalWrite(t *testing.T) {
	idx, root := mutationFixture(t)
	if err := writeFile(idx.Name, "/", "/file", strings.NewReader("saved")); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "file"))
	if err != nil || string(b) != "saved" {
		t.Fatalf("%q: %v", b, err)
	}
	// A subsequent lease must succeed after the writer has returned.
	release, err := acquireMutation(mutationTarget{index: idx, path: filepath.Join(root, "file")})
	if err != nil {
		t.Fatal(err)
	}
	release()
}

func TestMutationGuardFailedMoveReleasesSource(t *testing.T) {
	idx, root := mutationFixture(t)
	src, dst := filepath.Join(root, "a"), filepath.Join(root, "z")
	clientLock(t, idx, dst)
	_, err := acquireMutation(mutationTarget{index: idx, path: src}, mutationTarget{index: idx, path: dst})
	if !errors.Is(err, webdav.ErrLocked) {
		t.Fatalf("%v", err)
	}
	token, err := idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: filepath.ToSlash(src), Duration: -1})
	if err != nil {
		t.Fatalf("partial move lease leaked: %v", err)
	}
	_ = idx.WebdavLock.Unlock(time.Now(), token)
}

type blockingMutationReader struct {
	started chan struct{}
	finish  chan struct{}
	read    bool
}

func (r *blockingMutationReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.EOF
	}
	r.read = true
	close(r.started)
	<-r.finish
	return copy(p, "saved"), nil
}

func TestMutationGuardHoldsThroughWrite(t *testing.T) {
	idx, root := mutationFixture(t)
	r := &blockingMutationReader{started: make(chan struct{}), finish: make(chan struct{})}
	done := make(chan error, 1)
	go func() { done <- writeFile(idx.Name, "/", "/file", r) }()
	select {
	case <-r.started:
	case err := <-done:
		t.Fatalf("writer ended early: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("writer did not start")
	}
	_, err := idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: filepath.ToSlash(filepath.Join(root, "file")), Duration: -1})
	close(r.finish)
	writeErr := <-done
	if !errors.Is(err, webdav.ErrLocked) {
		t.Fatalf("writer did not retain lease: %v", err)
	}
	if writeErr != nil {
		t.Fatal(writeErr)
	}
}

func TestMutationGuardAllowsSourceRootAlias(t *testing.T) {
	idx, root := mutationFixture(t)
	parent, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "source")
	if err = os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	idx.Path = alias
	key, err := mutationLockPath(idx, filepath.Join(alias, "file"))
	if err != nil || key != filepath.ToSlash(filepath.Join(root, "file")) {
		t.Fatalf("%q: %v", key, err)
	}
}

func TestMutationGuardUsesSharedIndexGate(t *testing.T) {
	idx, root := mutationFixture(t)
	unlock := indexing.LockDAVMutations(idx)
	done := make(chan error, 1)
	started := make(chan struct{})
	go func() { close(started); done <- writeFile(idx.Name, "/", "/file", strings.NewReader("saved")) }()
	<-started
	select {
	case err := <-done:
		t.Fatalf("writer passed held WebDAV gate: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if _, err := os.Stat(filepath.Join(root, "file")); !os.IsNotExist(err) {
		t.Fatalf("write started while gate held: %v", err)
	}
	unlock()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writer stayed blocked")
	}
}
