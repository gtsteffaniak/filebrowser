package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	liberrors "github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing"
)

func setupSymlinkDirEscapeFixture(t *testing.T) (sourceRoot, userScope string) {
	t.Helper()
	root := t.TempDir()
	sourceRoot = filepath.Join(root, "srv")
	userDir := filepath.Join(sourceRoot, "user1")
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatal(err)
	}
	otherDir := filepath.Join(sourceRoot, "otheruser")
	if err := os.MkdirAll(otherDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherDir, "leaked.txt"), []byte("leaked"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(otherDir, filepath.Join(userDir, "escape_dir")); err != nil {
		t.Fatal(err)
	}
	return sourceRoot, "/user1"
}

func TestGetDirItems_RejectsDirectorySymlinkOutsideUserScope(t *testing.T) {
	sourceRoot, userScope := setupSymlinkDirEscapeFixture(t)
	sourceName := "test_getdiritems_scope"
	indexing.SetTestIndex(sourceName, sourceRoot)
	t.Cleanup(indexing.ClearTestIndices)

	origCheck := CheckPermissionsFunc
	t.Cleanup(func() { CheckPermissionsFunc = origCheck })
	CheckPermissionsFunc = func(opts utils.FileOptions, _ *access.Storage, _ *users.User) (string, string, error) {
		return utils.JoinPathAsUnix(userScope, opts.Path), userScope, nil
	}

	_, err := GetDirItems(utils.FileOptions{
		Source:         sourceName,
		Path:           "/escape_dir",
		FollowSymlinks: true,
	}, nil, &users.User{})
	if !errors.Is(err, liberrors.ErrPathEscapesScope) {
		t.Fatalf("expected ErrPathEscapesScope, got %v", err)
	}
}
