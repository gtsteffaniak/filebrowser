package sql

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

func TestUpdateFolderSizesIfChangedBindsAllParameters(t *testing.T) {
	restore := pushTestIndexConfig(t, t.TempDir(), testIndexSQLConfigNoFTS(settings.IndexStartupIntegrityOff))
	t.Cleanup(restore)
	db, _, err := NewIndexDB("folder_sizes", "OFF", 1000, 8, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	const source = "folder-sizes"
	wanted := map[string]uint64{"/": 1234, "/one/": 1000, "/one/empty/": 0}
	for path := range wanted {
		row := &iteminfo.FileInfo{
			Path: path, IsDir: true,
			ItemInfo: iteminfo.ItemInfo{Name: path, Type: "directory", Size: 999},
		}
		require.NoError(t, db.InsertItem(source, path, row))
	}
	other := &iteminfo.FileInfo{
		Path: "/one/", IsDir: true,
		ItemInfo: iteminfo.ItemInfo{Name: "one", Type: "directory", Size: 777},
	}
	require.NoError(t, db.InsertItem("other-source", other.Path, other))

	updated, err := db.UpdateFolderSizesIfChanged(source, wanted)
	require.NoError(t, err)
	require.EqualValues(t, len(wanted), updated)
	for path, size := range wanted {
		row, err := db.GetItem(source, path)
		require.NoError(t, err)
		require.NotNil(t, row)
		require.EqualValues(t, size, row.Size)
	}
	row, err := db.GetItem("other-source", other.Path)
	require.NoError(t, err)
	require.NotNil(t, row)
	require.EqualValues(t, 777, row.Size)

	updated, err = db.UpdateFolderSizesIfChanged(source, wanted)
	require.NoError(t, err)
	require.Zero(t, updated)
}
