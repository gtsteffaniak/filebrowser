package sqldb

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

// An existing database at schema 4 is upgraded to 5, which clears the preview
// cache once. Reopening at schema 5 leaves new previews in place.
func TestUpgradeFromBefore22ClearsPreviewCacheOnce(t *testing.T) {
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0o644, 0o755)
	}
	prev := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = t.TempDir()
	t.Cleanup(func() { settings.Config.Server.CacheDir = prev })

	dbPath := filepath.Join(t.TempDir(), "database.db")
	store, _, err := NewSQLStoreWithOptions(dbPath, NewSQLStoreOpts{SkipQuickSetup: true})
	require.NoError(t, err)
	_, err = store.db.Exec("UPDATE schema_version SET version = 4")
	require.NoError(t, err)
	require.NoError(t, store.db.Close())

	previewDir := settings.PreviewDiskCacheDir()
	require.NoError(t, os.MkdirAll(previewDir, fileutils.PermDir))
	oldPreview := filepath.Join(previewDir, "old.jpg")
	require.NoError(t, os.WriteFile(oldPreview, []byte("old"), 0o644))

	store, _, err = NewSQLStoreWithOptions(dbPath, NewSQLStoreOpts{SkipQuickSetup: true})
	require.NoError(t, err)
	version, err := getSchemaVersion(store.db)
	require.NoError(t, err)
	require.Equal(t, currentSchemaVersion, version)
	_, err = os.Stat(oldPreview)
	require.True(t, os.IsNotExist(err), "preview from before v2.2.0 is cleared")

	newPreview := filepath.Join(previewDir, "new.jpg")
	require.NoError(t, os.WriteFile(newPreview, []byte("new"), 0o644))
	require.NoError(t, store.db.Close())

	store, _, err = NewSQLStoreWithOptions(dbPath, NewSQLStoreOpts{SkipQuickSetup: true})
	require.NoError(t, err)
	defer store.db.Close()
	_, err = os.Stat(newPreview)
	require.NoError(t, err, "a reopen at the current schema does not clear again")
}
