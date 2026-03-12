package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

// Index info operations

// GetIndexInfo retrieves index info by path
// Returns a value (not pointer) to prevent modifications to the cache
func GetIndexInfo(path string) (dbindex.IndexInfo, error) {
	indexMux.RLock()
	defer indexMux.RUnlock()

	info, exists := indexInfoByPath[path]
	if !exists {
		return dbindex.IndexInfo{}, errors.ErrNotExist
	}

	// Return a value copy - automatically immutable
	return copyIndexInfo(info), nil
}

// GetIndexInfoBySource retrieves all index info for a source
// Returns values (not pointers) to prevent modifications to the cache
func GetIndexInfoBySource(source string) ([]dbindex.IndexInfo, error) {
	indexMux.RLock()
	defer indexMux.RUnlock()

	var infoList []dbindex.IndexInfo
	for _, info := range indexInfoByPath {
		if info.Source == source {
			infoList = append(infoList, copyIndexInfo(info))
		}
	}
	return infoList, nil
}

// GetAllIndexInfo retrieves all index info
// Returns values (not pointers) to prevent modifications to the cache
func GetAllIndexInfo() ([]dbindex.IndexInfo, error) {
	indexMux.RLock()
	defer indexMux.RUnlock()

	infoList := make([]dbindex.IndexInfo, 0, len(indexInfoByPath))
	for _, info := range indexInfoByPath {
		infoList = append(infoList, copyIndexInfo(info))
	}
	return infoList, nil
}

// CreateIndexInfo creates new index info with write-through to SQL
func CreateIndexInfo(info *dbindex.IndexInfo) error {
	indexMux.Lock()
	defer indexMux.Unlock()

	// 1. Check if index info already exists in cache (state)
	if _, exists := indexInfoByPath[info.Path]; exists {
		return fmt.Errorf("index info for path %s already exists", info.Path)
	}

	// 2. Write to database
	err := sqlStore.SaveIndexInfo(info)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	indexInfoByPath[info.Path] = info

	return nil
}

// UpdateIndexInfo updates existing index info with write-through to SQL
func UpdateIndexInfo(info *dbindex.IndexInfo) error {
	indexMux.Lock()
	defer indexMux.Unlock()

	// 1. Check if index info exists in cache (state)
	if _, exists := indexInfoByPath[info.Path]; !exists {
		return fmt.Errorf("index info for path %s not found in cache", info.Path)
	}

	// 2. Write to database
	err := sqlStore.SaveIndexInfo(info)
	if err != nil {
		return err
	}

	// 3. Update cache to match database
	indexInfoByPath[info.Path] = info

	return nil
}

// SaveIndexInfo saves index info with write-through to SQL
// DEPRECATED: Use CreateIndexInfo or UpdateIndexInfo instead
// This function is kept for backward compatibility but should not be used for new code
func SaveIndexInfo(info *dbindex.IndexInfo) error {
	indexMux.Lock()
	defer indexMux.Unlock()

	// Write through to SQL
	err := sqlStore.SaveIndexInfo(info)
	if err != nil {
		return err
	}

	// Update cache
	indexInfoByPath[info.Path] = info

	return nil
}

// DeleteIndexInfo deletes index info by path
func DeleteIndexInfo(path string) error {
	indexMux.Lock()
	defer indexMux.Unlock()

	// 1. Check if index info exists in cache (state)
	if _, exists := indexInfoByPath[path]; !exists {
		return fmt.Errorf("index info for path %s not found in cache", path)
	}

	// 2. Delete from database
	err := sqlStore.DeleteIndexInfo(path)
	if err != nil {
		return err
	}

	// 3. Remove from cache to match database
	delete(indexInfoByPath, path)

	return nil
}

// copyIndexInfo creates a deep copy of an index info object and returns a value
func copyIndexInfo(info *dbindex.IndexInfo) dbindex.IndexInfo {
	infoCopy := *info

	// Deep copy the scanners map
	if info.Scanners != nil {
		infoCopy.Scanners = make(map[string]*dbindex.PersistedScannerInfo, len(info.Scanners))
		for k, v := range info.Scanners {
			if v != nil {
				scannerCopy := *v
				infoCopy.Scanners[k] = &scannerCopy
			}
		}
	}

	return infoCopy
}
