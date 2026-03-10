package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

// Index info operations

// GetIndexInfo retrieves index info by path
func GetIndexInfo(path string) (*dbindex.IndexInfo, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	info, exists := indexInfoByPath[path]
	if !exists {
		return nil, fmt.Errorf("index info not found")
	}
	return info, nil
}

// GetIndexInfoBySource retrieves all index info for a source
func GetIndexInfoBySource(source string) ([]*dbindex.IndexInfo, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	var infoList []*dbindex.IndexInfo
	for _, info := range indexInfoByPath {
		if info.Source == source {
			infoList = append(infoList, info)
		}
	}
	return infoList, nil
}

// GetAllIndexInfo retrieves all index info
func GetAllIndexInfo() ([]*dbindex.IndexInfo, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	infoList := make([]*dbindex.IndexInfo, 0, len(indexInfoByPath))
	for _, info := range indexInfoByPath {
		infoList = append(infoList, info)
	}
	return infoList, nil
}

// SaveIndexInfo saves index info with write-through to SQL
func SaveIndexInfo(info *dbindex.IndexInfo) error {
	mux.Lock()
	defer mux.Unlock()
	
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
	mux.Lock()
	defer mux.Unlock()
	
	// Delete from SQL
	err := sqlStore.DeleteIndexInfo(path)
	if err != nil {
		return err
	}
	
	// Remove from cache
	delete(indexInfoByPath, path)
	
	return nil
}
