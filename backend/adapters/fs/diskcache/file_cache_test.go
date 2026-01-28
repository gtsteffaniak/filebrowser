package diskcache

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/stretchr/testify/require"
)

func TestFileCache(t *testing.T) {
	ctx := context.Background()
	const (
		key            = "key"
		value          = "some text"
		newValue       = "new text"
		cacheRoot      = "cache"
		cachedFilePath = "a/62/a62f2225bf70bfaccbc7f1ef2a397836717377de"
	)

	// Set up file permissions before creating cache
	fileutils.SetFsPermissions(0644, 0755)

	// Create temporary directory for the cache
	cacheDir, err := os.MkdirTemp("", cacheRoot)
	require.NoError(t, err)
	defer os.RemoveAll(cacheDir) // Clean up

	cache, err := NewFileCache(cacheDir)
	require.NoError(t, err)

	// store new key
	// Note: NewFileCache creates a "diskcache" subdirectory, so the actual path includes it
	err = cache.Store(ctx, key, []byte(value))
	require.NoError(t, err)
	checkValue(t, ctx, cache, filepath.Join(cacheDir, "diskcache", cachedFilePath), key, value)

	// update existing key
	err = cache.Store(ctx, key, []byte(newValue))
	require.NoError(t, err)
	checkValue(t, ctx, cache, filepath.Join(cacheDir, "diskcache", cachedFilePath), key, newValue)

	// delete key
	err = cache.Delete(ctx, key)
	require.NoError(t, err)
	exists := fileExists(filepath.Join(cacheDir, "diskcache", cachedFilePath))
	require.False(t, exists)
}

func checkValue(t *testing.T, ctx context.Context, cache *FileCache, fileFullPath string, key, wantValue string) {
	t.Helper()
	// check actual file content
	b, err := os.ReadFile(fileFullPath)
	require.NoError(t, err)
	require.Equal(t, wantValue, string(b))

	// check cache content
	b, ok, err := cache.Load(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, wantValue, string(b))
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
