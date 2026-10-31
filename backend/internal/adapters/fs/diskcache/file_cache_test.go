package diskcache

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
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

// storeAged stores a value and sets its modification time, so the eviction order is deterministic.
func storeAged(t *testing.T, cache *FileCache, key string, size int, age time.Duration) {
	t.Helper()
	require.NoError(t, cache.Store(context.Background(), key, make([]byte, size)))
	when := time.Now().Add(-age)
	require.NoError(t, os.Chtimes(cache.getFileName(key), when, when))
}

func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func newLimitedCache(t *testing.T, maxBytes int64) *FileCache {
	t.Helper()
	fileutils.SetFsPermissions(0644, 0755)
	cache, err := NewFileCacheWithLimit(t.TempDir(), maxBytes)
	require.NoError(t, err)
	return cache
}

func TestFileCacheEvictsLeastRecentlyUsedOverLimit(t *testing.T) {
	const kb = 1024
	cache := newLimitedCache(t, 10*kb)
	// 3 files of 3KB are under the 10KB limit
	storeAged(t, cache, "oldest", 3*kb, 3*time.Hour)
	storeAged(t, cache, "old", 3*kb, 2*time.Hour)
	storeAged(t, cache, "recent", 3*kb, time.Hour)
	require.True(t, fileExists(cache.getFileName("oldest")), "nothing is removed under the limit")

	// the 4th file goes over the limit: the oldest ones are removed until it is under 90% of it
	require.NoError(t, cache.Store(context.Background(), "newest", make([]byte, 3*kb)))
	waitFor(t, func() bool { return !fileExists(cache.getFileName("oldest")) })
	waitFor(t, func() bool { return !cache.evicting.Load() })

	require.False(t, fileExists(cache.getFileName("oldest")))
	require.False(t, fileExists(cache.getFileName("old")))
	require.True(t, fileExists(cache.getFileName("recent")))
	require.True(t, fileExists(cache.getFileName("newest")))
	require.LessOrEqual(t, cache.size.Load(), int64(9*kb))
}

func TestFileCacheTrimsAnOversizedCacheAtStartup(t *testing.T) {
	const kb = 1024
	fileutils.SetFsPermissions(0644, 0755)
	dir := t.TempDir()
	unlimited, err := NewFileCache(dir)
	require.NoError(t, err)
	storeAged(t, &FileCache{dir: unlimited.dir}, "a", 4*kb, 4*time.Hour)
	storeAged(t, &FileCache{dir: unlimited.dir}, "b", 4*kb, 3*time.Hour)
	storeAged(t, &FileCache{dir: unlimited.dir}, "c", 4*kb, 2*time.Hour)

	cache, err := NewFileCacheWithLimit(dir, 10*kb)
	require.NoError(t, err)

	// 12KB over a 10KB limit: only the oldest file is removed, which already brings it under 90% (9KB)
	require.False(t, fileExists(cache.getFileName("a")))
	require.True(t, fileExists(cache.getFileName("b")))
	require.True(t, fileExists(cache.getFileName("c")))
	require.Equal(t, int64(8*kb), cache.size.Load())
}

func TestFileCacheWithoutLimitKeepsEverything(t *testing.T) {
	fileutils.SetFsPermissions(0644, 0755)
	cache, err := NewFileCache(t.TempDir())
	require.NoError(t, err)
	for i := 0; i < 5; i++ {
		storeAged(t, cache, fmt.Sprintf("key-%d", i), 1<<20, time.Duration(i)*time.Hour)
	}
	for i := 0; i < 5; i++ {
		require.True(t, fileExists(cache.getFileName(fmt.Sprintf("key-%d", i))))
	}
	require.Zero(t, cache.size.Load(), "size is only tracked when there is a limit")
}

func TestFileCacheReadRefreshesModTime(t *testing.T) {
	cache := newLimitedCache(t, 1<<20)
	storeAged(t, cache, "used", 100, 3*time.Hour)

	_, ok, err := cache.Load(context.Background(), "used")
	require.NoError(t, err)
	require.True(t, ok)

	info, err := os.Stat(cache.getFileName("used"))
	require.NoError(t, err)
	require.WithinDuration(t, time.Now(), info.ModTime(), time.Minute, "a file that is read is the last to be evicted")
}

func TestFileCacheDeleteAndOverwriteKeepTheSizeAccurate(t *testing.T) {
	const kb = 1024
	cache := newLimitedCache(t, 1<<20)
	ctx := context.Background()
	require.NoError(t, cache.Store(ctx, "k", make([]byte, 5*kb)))
	require.Equal(t, int64(5*kb), cache.size.Load())
	require.NoError(t, cache.Store(ctx, "k", make([]byte, 2*kb)))
	require.Equal(t, int64(2*kb), cache.size.Load(), "overwriting replaces the old size")
	require.NoError(t, cache.Delete(ctx, "k"))
	require.Zero(t, cache.size.Load())
}

// Stores that land while an eviction is measuring or deleting must not be lost from the tracked size,
// and the cache must end up under its limit even though more files kept arriving during the eviction.
func TestFileCacheStaysUnderTheLimitWithConcurrentStores(t *testing.T) {
	const (
		kb      = 1024
		limit   = 100 * kb
		workers = 16
		stores  = 40
	)
	cache := newLimitedCache(t, limit)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < stores; i++ {
				_ = cache.Store(context.Background(), fmt.Sprintf("worker-%d-key-%d", w, i), make([]byte, 4*kb))
			}
		}(w)
	}
	wg.Wait()
	waitFor(t, func() bool { return !cache.evicting.Load() })

	_, onDisk := cache.scan()
	require.LessOrEqual(t, onDisk, int64(limit), "the files left on disk are within the limit")
	require.Equal(t, onDisk, cache.size.Load(), "the tracked size matches what is on disk")
}
