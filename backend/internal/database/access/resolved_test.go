package access

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupSymlinkDenyFixture(t *testing.T) (sourceRoot string, idx *indexing.Index, store *Storage, username string) {
	t.Helper()
	sourceRoot = filepath.Join(t.TempDir(), "data")
	aliceDir := filepath.Join(sourceRoot, "alice")
	bobDir := filepath.Join(sourceRoot, "bob")
	for _, dir := range []string{aliceDir, bobDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(bobDir, "secret.txt"), []byte("secret"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../bob/secret.txt", filepath.Join(aliceDir, "link.txt")); err != nil {
		t.Skipf("symlinks required: %v", err)
	}
	if err := os.Symlink("../bob", filepath.Join(aliceDir, "bobdir")); err != nil {
		t.Skipf("symlinks required: %v", err)
	}

	settings.Config.Server.SourceMap = map[string]*settings.Source{
		sourceRoot: {Path: sourceRoot, Config: settings.SourceConfig{DenyByDefault: false}},
	}
	store = NewStorage(nil)
	username = "scoped-user"
	if err := store.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/bob", true), username); err != nil {
		t.Fatal(err)
	}
	idx = &indexing.Index{Source: settings.Source{Path: sourceRoot}}
	return sourceRoot, idx, store, username
}

func TestPermittedAtRealPath_AllowsLinkPathDeniesResolvedTarget(t *testing.T) {
	_, idx, store, user := setupSymlinkDenyFixture(t)

	if !store.Permitted(idx.Path, utils.IndexPathFromNormalized("/alice/link.txt", false), user) {
		t.Fatal("index path at the symlink should be allowed")
	}
	if store.Permitted(idx.Path, utils.IndexPathFromNormalized("/bob/secret.txt", false), user) {
		t.Fatal("direct path under /bob should be denied")
	}

	realPath, _, err := idx.GetRealPathScoped("/", "/alice/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if store.PermittedAtRealPath(idx, user, realPath, false) {
		t.Fatal("resolved target under denied /bob must not be permitted")
	}
}

func TestPermittedAtRealPath_DeniesWriteThroughDirectorySymlink(t *testing.T) {
	_, idx, store, user := setupSymlinkDenyFixture(t)

	realPath, err := idx.GetRealPathForMutation("/", "/alice/bobdir/planted.txt")
	if err != nil {
		t.Fatal(err)
	}
	if store.PermittedAtRealPath(idx, user, realPath, false) {
		t.Fatal("resolved write target under denied /bob must not be permitted")
	}
}

func TestPermittedAtRealPath_DeniesCopySourceResolved(t *testing.T) {
	_, idx, store, user := setupSymlinkDenyFixture(t)

	realSrc, isDir, err := idx.GetRealPathScoped("/", "/alice/link.txt")
	if err != nil {
		t.Fatal(err)
	}
	if isDir {
		t.Fatal("expected file")
	}
	if store.PermittedAtRealPath(idx, user, realSrc, isDir) {
		t.Fatal("copy source resolved to denied target must not be permitted")
	}
}

func TestIsDirAtRealPath(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsDirAtRealPath(dir) {
		t.Fatal("expected directory")
	}
	if IsDirAtRealPath(file) {
		t.Fatal("expected file")
	}
	if IsDirAtRealPath(filepath.Join(dir, "missing")) {
		t.Fatal("missing path should not be treated as directory")
	}
}
