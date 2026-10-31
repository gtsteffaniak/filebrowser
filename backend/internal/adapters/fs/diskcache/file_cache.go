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
	// size is the approximate total size of the cached files in bytes. Only tracked when maxBytes > 0.
	size atomic.Int64
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
		files, total := cache.scan()
		cache.size.Store(total)
		if total > maxBytes {
			logger.Infof("preview cache is %d MB, over its limit of %d MB, removing the least recently used files", total>>20, maxBytes>>20)
			cache.evict(files, total)
		}
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

	var previousSize int64
	if f.maxBytes > 0 {
		if info, err := os.Stat(fileName); err == nil {
			previousSize = info.Size()
		}
	}

	if err := os.WriteFile(fileName, value, fileutils.PermFile); err != nil {
		return err
	}

	if f.maxBytes > 0 && f.size.Add(int64(len(value))-previousSize) > f.maxBytes {
		f.evictAsync()
	}
	return nil
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

	fileName := f.getFileName(key)
	var removedSize int64
	if f.maxBytes > 0 {
		if info, err := os.Stat(fileName); err == nil {
			removedSize = info.Size()
		}
	}
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
		files, total := f.scan()
		f.size.Store(total)
		f.evict(files, total)
	}()
}

// evict deletes the least recently used files until the cache is under evictTargetPercent of its limit.
// Deleting a file that is being read or written is harmless: readers keep their open file and a
// removed entry is simply rebuilt the next time it is requested.
func (f *FileCache) evict(files []cachedFile, total int64) {
	f.evicting.Store(true)
	defer f.evicting.Store(false)
	target := f.maxBytes / 100 * evictTargetPercent
	if total <= target {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].modTime.Before(files[j].modTime) })
	var removed int
	var removedBytes int64
	for _, file := range files {
		if total <= target {
			break
		}
		if err := os.Remove(file.path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				total -= file.size
			}
			continue
		}
		total -= file.size
		removed++
		removedBytes += file.size
	}
	f.size.Store(total)
	logger.Debugf("preview cache limit: removed %d files (%d MB), now %d MB of %d MB", removed, removedBytes>>20, total>>20, f.maxBytes>>20)
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
