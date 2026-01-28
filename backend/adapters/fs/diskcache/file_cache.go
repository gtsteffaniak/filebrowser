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
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// Cache interface for caching operations
type Cache interface {
	Get(key string) ([]byte, error)
	Set(key string, value []byte) error
}

// FileCache struct for file-based caching
type FileCache struct {
	dir string
	// granular locks
	scopedLocks struct {
		sync.Mutex
		sync.Once
		locks map[string]sync.Locker
	}
}

func NewFileCache(dir string) (*FileCache, error) {
	cacheDir := filepath.Join(dir, "diskcache")

	// Migrate existing cache files from old structure (if any)
	if err := migrateOldCacheStructure(dir, cacheDir); err != nil {
		logger.Warningf("failed to migrate old cache structure: %v\n", err)
	}

	// Create the cache directory immediately with proper permissions
	if err := os.MkdirAll(cacheDir, fileutils.PermDir); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}
	return &FileCache{dir: cacheDir}, nil
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

	if err := os.WriteFile(fileName, value, fileutils.PermFile); err != nil {
		return err
	}

	return nil
}

func (f *FileCache) Load(ctx context.Context, key string) (value []byte, exist bool, err error) {
	r, ok, err := f.open(key)
	if err != nil || !ok {
		return nil, ok, err
	}
	defer r.Close()

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
	if err := os.Remove(fileName); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
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
