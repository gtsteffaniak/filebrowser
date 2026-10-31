package diskcache

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Cache interface for caching operations
type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}

const (
	// evictTargetPercent is the size, as a percentage of the limit, that eviction shrinks the cache to.
	// Stopping below the limit avoids running an eviction on every write once the cache is full.
	evictTargetPercent = 90
	// touchInterval is how old a file's modification time has to be before a read refreshes it. Reads
	// refresh it so the files in use are the last ones evicted, but not on every read.
	touchInterval = time.Hour
)

// FileCache struct for file-based caching
type FileCache struct {
	dir string
	// maxBytes is the size limit of the cache in bytes. 0 means no limit.
	maxBytes int64
	// size is the total size of the cached files in bytes. Only tracked when maxBytes > 0.
	size atomic.Int64
	// sizeMu keeps size consistent with the files on disk: stores and deletes hold it for reading while
	// they change a file and its size, and eviction holds it for writing while it measures the directory,
	// so a store can never fall between a measurement and the size being set from it.
	sizeMu sync.RWMutex
	// evicting is set while an eviction is running so only one runs at a time.
	evicting atomic.Bool
	// granular locks
	scopedLocks struct {
		sync.Mutex
		sync.Once
		locks map[string]sync.Locker
	}
}

func NewFileCache(dir string) (*FileCache, error) {
	return NewFileCacheWithLimit(dir, 0)
}

// NewFileCacheWithLimit is like NewFileCache but keeps the cache under maxBytes, deleting the least
// recently used files when it grows past it. 0 means no limit.
func NewFileCacheWithLimit(dir string, maxBytes int64) (*FileCache, error) {
	cacheDir := filepath.Join(dir, "diskcache")

	// Migrate existing cache files from old structure (if any)
	if err := migrateOldCacheStructure(dir, cacheDir); err != nil {
		logger.Warningf("failed to migrate old cache structure: %v\n", err)
	}

	// Create the cache directory immediately with proper permissions
	if err := os.MkdirAll(cacheDir, fileutils.PermDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	cache := &FileCache{dir: cacheDir}
	if maxBytes > 0 {
		cache.maxBytes = maxBytes
		// measures the cache and trims it right away if it is already over the limit
		cache.evictPass()
	}
	return cache, nil
}

// migrateOldCacheStructure moves cache files from old structure (dir/a/...) to new structure (dir/diskcache/a/...)
func migrateOldCacheStructure(oldDir, newDir string) error {
	// Read the old cache directory
	entries, err := os.ReadDir(oldDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist yet, nothing to migrate
		}
		return err
	}

	migrated := 0
	for _, entry := range entries {
		// Only migrate single-character directories (cache hash directories like a, b, c, etc.)
		if !entry.IsDir() || len(entry.Name()) != 1 {
			continue
		}

		// Skip if it's already the diskcache directory
		if entry.Name() == "d" && filepath.Join(oldDir, entry.Name()) == newDir {
			continue
		}

		oldPath := filepath.Join(oldDir, entry.Name())
		newPath := filepath.Join(newDir, entry.Name())

		// Create parent directory in new location
		if err := os.MkdirAll(newDir, fileutils.PermDir); err != nil {
			return err
		}

		// Move the directory
		if err := os.Rename(oldPath, newPath); err != nil {
			// If rename fails (e.g., cross-device), skip this one
			continue
		}
		migrated++
	}

	return nil
}

func (f *FileCache) Store(ctx context.Context, key string, value []byte) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	fileName := f.getFileName(key)
	if err := os.MkdirAll(filepath.Dir(fileName), fileutils.PermDir); err != nil {
		return err
	}

	if f.maxBytes > 0 {
		f.sizeMu.RLock()
		defer f.sizeMu.RUnlock()
	}
	previousSize := f.fileSize(fileName)

	if err := os.WriteFile(fileName, value, fileutils.PermFile); err != nil {
		// a failed write can leave a truncated or partial file, so count what is actually there
		f.sizeChanged(f.fileSize(fileName) - previousSize)
		return err
	}
	f.sizeChanged(int64(len(value)) - previousSize)
	return nil
}

// fileSize returns the size of a cached file, or 0 if it doesn't exist or the size isn't tracked.
func (f *FileCache) fileSize(fileName string) int64 {
	if f.maxBytes <= 0 {
		return 0
	}
	info, err := os.Stat(fileName)
	if err != nil {
		return 0
	}
	return info.Size()
}

// sizeChanged adds delta to the tracked size and starts an eviction if that put the cache over its limit.
func (f *FileCache) sizeChanged(delta int64) {
	if f.maxBytes <= 0 {
		return
	}
	if f.size.Add(delta) > f.maxBytes {
		f.evictAsync()
	}
}

func (f *FileCache) Load(ctx context.Context, key string) (value []byte, exist bool, err error) {
	r, ok, err := f.open(key)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer r.Close()
	f.touch(r)

	value, err = io.ReadAll(r)
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (f *FileCache) Delete(ctx context.Context, key string) error {
	mu := f.getScopedLocks(key)
	mu.Lock()
	defer mu.Unlock()

	if f.maxBytes > 0 {
		f.sizeMu.RLock()
		defer f.sizeMu.RUnlock()
	}
	fileName := f.getFileName(key)
	removedSize := f.fileSize(fileName)
	if err := os.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if removedSize > 0 {
		f.size.Add(-removedSize)
	}
	return nil
}

// touch refreshes the modification time of a file that was just read, which is what eviction orders by.
func (f *FileCache) touch(file *os.File) {
	if f.maxBytes <= 0 {
		return
	}
	info, err := file.Stat()
	if err != nil || time.Since(info.ModTime()) < touchInterval {
		return
	}
	now := time.Now()
	_ = os.Chtimes(file.Name(), now, now)
}

type cachedFile struct {
	path    string
	size    int64
	modTime time.Time
}

// scan lists the cached files and their total size.
func (f *FileCache) scan() ([]cachedFile, int64) {
	var files []cachedFile
	var total int64
	_ = filepath.WalkDir(f.dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		files = append(files, cachedFile{path: path, size: info.Size(), modTime: info.ModTime()})
		total += info.Size()
		return nil
	})
	return files, total
}

// evictAsync starts an eviction in the background unless one is already running, so the request that
// stored the file doesn't wait for it.
func (f *FileCache) evictAsync() {
	if !f.evicting.CompareAndSwap(false, true) {
		return
	}
	go func() {
		for {
			removed := f.evictPass()
			f.evicting.Store(false)
			if removed == 0 {
				return // under the limit, or nothing could be removed: don't spin
			}
			// Files stored while the pass was running were counted but not necessarily seen by it, and a
			// store that came in just before the flag was cleared saw it set and didn't start another
			// pass, so check again before stopping.
			if f.size.Load() <= f.maxBytes || !f.evicting.CompareAndSwap(false, true) {
				return
			}
		}
	}()
}

// evictPass measures the cache and, if it is over the limit, deletes the least recently used files until
// it is under evictTargetPercent of the limit. Deleting a file that is being read or written is harmless:
// readers keep their open file and a removed entry is simply rebuilt the next time it is requested.
func (f *FileCache) evictPass() (removed int) {
	f.sizeMu.Lock()
	files, total := f.scan()
	f.size.Store(total)
	f.sizeMu.Unlock()

	if total <= f.maxBytes {
		return 0
	}
	target := f.maxBytes / 100 * evictTargetPercent
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	var removedBytes int64
	for _, file := range files {
		if f.size.Load() <= target {
			break
		}
		if err := os.Remove(file.path); err != nil {
			continue
		}
		f.size.Add(-file.size)
		removed++
		removedBytes += file.size
	}
	logger.Debugf("preview cache limit: removed %d files (%d MB), now %d MB of %d MB", removed, removedBytes>>20, f.size.Load()>>20, f.maxBytes>>20)
	return removed
}

func (f *FileCache) open(key string) (*os.File, bool, error) {
	fileName := f.getFileName(key)
	file, err := os.Open(fileName)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return file, true, nil
}

// getScopedLocks pull lock from the map if found or create a new one
func (f *FileCache) getScopedLocks(key string) (lock sync.Locker) {
	f.scopedLocks.Do(func() { f.scopedLocks.locks = map[string]sync.Locker{} })

	f.scopedLocks.Lock()
	lock, ok := f.scopedLocks.locks[key]
	if !ok {
		lock = &sync.Mutex{}
		f.scopedLocks.locks[key] = lock
	}
	f.scopedLocks.Unlock()

	return lock
}

func (f *FileCache) getFileName(key string) string {
	hasher := sha1.New()
	_, _ = hasher.Write([]byte(key))
	hash := hex.EncodeToString(hasher.Sum(nil))
	return filepath.Join(f.dir, fmt.Sprintf("%s/%s/%s", hash[:1], hash[1:3], hash))
}
