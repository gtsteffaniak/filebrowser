package indexing

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	liberrors "github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupSymlinkEscapeFixture(t *testing.T) (sourceRoot, userScope string) {
	t.Helper()
	root := t.TempDir()
	sourceRoot = filepath.Join(root, "srv")
	userDir := filepath.Join(sourceRoot, "user1")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	outsideSecret := filepath.Join(root, "secret-outside.txt")
	if err := os.WriteFile(outsideSecret, []byte("ROOT_DB_PASSWORD=supersecret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outsideSecret, filepath.Join(userDir, "secret_link")); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(root, "outside-sibling.txt")
	if err := os.WriteFile(sibling, []byte("outside sibling"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(sibling, filepath.Join(userDir, "sibling_link")); err != nil {
		t.Fatal(err)
	}
	inside := filepath.Join(userDir, "allowed.txt")
	if err := os.WriteFile(inside, []byte("allowed"), 0o644); err != nil {
		t.Fatal(err)
	}
	return sourceRoot, "/user1"
}

func symlinkTestIndex(sourceRoot string) *Index {
	return &Index{Source: settings.Source{Path: sourceRoot}}
}

func TestPathWithinRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "inner", "file.txt")
	if err := os.MkdirAll(filepath.Dir(inside), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(outside, []byte("y"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := iteminfo.PathWithinRoot(root, inside); err != nil {
		t.Fatalf("expected inside path to be allowed: %v", err)
	}
	if err := iteminfo.PathWithinRoot(root, outside); err == nil {
		t.Fatal("expected outside path to be rejected")
	}
}

func TestGetRealPath_RejectsEscapeOutsideSource(t *testing.T) {
	sourceRoot, _ := setupSymlinkEscapeFixture(t)
	idx := symlinkTestIndex(sourceRoot)

	_, _, err := idx.GetRealPath("/user1/secret_link")
	if !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope, got %v", err)
	}
	_, _, err = idx.GetRealPath("/user1/sibling_link")
	if !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope for sibling link, got %v", err)
	}
}

func TestGetRealPathScoped_RejectsEscapeOutsideUserScope(t *testing.T) {
	sourceRoot, userScope := setupSymlinkEscapeFixture(t)
	idx := symlinkTestIndex(sourceRoot)

	// Under source but outside /user1: place secret inside source root
	insideSource := filepath.Join(sourceRoot, "secret-in-source.txt")
	if err := os.WriteFile(insideSource, []byte("in source"), 0o644); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(sourceRoot, "user1")
	if err := os.Symlink(insideSource, filepath.Join(userDir, "in_source_link")); err != nil {
		t.Fatal(err)
	}

	_, _, err := idx.GetRealPathScoped(userScope, "/user1/in_source_link")
	if !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope for in-source escape, got %v", err)
	}
}

func TestGetRealPathScoped_AllowsInScopeTarget(t *testing.T) {
	sourceRoot, userScope := setupSymlinkEscapeFixture(t)
	idx := symlinkTestIndex(sourceRoot)

	userDir := filepath.Join(sourceRoot, "user1")
	if err := os.Symlink("allowed.txt", filepath.Join(userDir, "good_link")); err != nil {
		t.Fatal(err)
	}

	realPath, _, err := idx.GetRealPathScoped(userScope, "/user1/good_link")
	if err != nil {
		t.Fatalf("expected in-scope symlink to resolve: %v", err)
	}
	want := filepath.Join(userDir, "allowed.txt")
	realPath, err = filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatal(err)
	}
	want, err = filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	if realPath != want {
		t.Fatalf("got real path %q, want %q", realPath, want)
	}
}

func TestGetRealPathScoped_CacheKeyDoesNotCollideWithUnscoped(t *testing.T) {
	sourceRoot, userScope := setupSymlinkEscapeFixture(t)
	idx := symlinkTestIndex(sourceRoot)

	insideSource := filepath.Join(sourceRoot, "secret-in-source.txt")
	if err := os.WriteFile(insideSource, []byte("in source"), 0o644); err != nil {
		t.Fatal(err)
	}
	userDir := filepath.Join(sourceRoot, "user1")
	if err := os.WriteFile(filepath.Join(userDir, "foo"), []byte("legitimate"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Legacy scoped cache keys used joinedPath + ":bound:" + bound, which could collide
	// with an unscoped joinedPath when ":bound:" appeared in a path component.
	legacyKey := filepath.Join(sourceRoot, "user1", "foo") + ":bound:" + "/user1"
	RealPathCache.Set(legacyKey, insideSource)
	IsDirCache.Set(legacyKey+":isdir", false)

	realPath, _, err := idx.GetRealPathScoped(userScope, "/user1/foo")
	if err != nil {
		t.Fatalf("scoped in-scope file should resolve: %v", err)
	}
	want := filepath.Join(userDir, "foo")
	want, err = filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatal(err)
	}
	realPath, err = filepath.EvalSymlinks(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if realPath != want {
		t.Fatalf("scoped lookup returned %q, want in-scope file %q", realPath, want)
	}
}

func TestGetFileInfo_RejectsSymlinkEscapeWithBound(t *testing.T) {
	sourceRoot, userScope := setupSymlinkEscapeFixture(t)
	idx := symlinkTestIndex(sourceRoot)

	_, err := idx.GetFileInfo(FileInfoRequest{
		IndexPath:      "/user1/secret_link",
		FollowSymlinks: true,
		BoundIndexPath: userScope,
	})
	if !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope from GetFileInfo, got %v", err)
	}
}
