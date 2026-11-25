package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

func TestCachedZeroFolderSizeOverridesStoredSize(t *testing.T) {
	idx, _, cleanup := setupTestIndex(t)
	t.Cleanup(cleanup)

	for path, name := range map[string]string{"/": "", "/empty/": "empty", "/cold/": "cold"} {
		row := &iteminfo.FileInfo{
			Path: path, IsDir: true,
			ItemInfo: iteminfo.ItemInfo{Name: name, Type: "directory", Size: 999},
		}
		require.NoError(t, idx.db.InsertItem(idx.Name, path, row))
	}
	idx.SetFolderSize("/", 0)
	idx.SetFolderSize("/empty/", 0)

	empty, ok := idx.GetReducedMetadata("/empty/", true)
	require.True(t, ok)
	require.Zero(t, empty.Size)
	cold, ok := idx.GetReducedMetadata("/cold/", true)
	require.True(t, ok)
	require.EqualValues(t, 999, cold.Size)

	parent, ok := idx.GetMetadataInfo("/", true, false)
	require.True(t, ok)
	require.Zero(t, parent.Size)
	sizes := make(map[string]int64)
	for _, child := range parent.Folders {
		sizes[child.Name] = child.Size
	}
	require.Subset(t, sizes, map[string]int64{"empty": 0, "cold": 999})
}
