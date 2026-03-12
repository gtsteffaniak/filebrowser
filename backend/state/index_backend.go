package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

// indexBackend implements dbindex.StorageBackend using state
type indexBackend struct{}

func (i indexBackend) All() ([]*dbindex.IndexInfo, error) {
	infoList, err := GetAllIndexInfo()
	if err != nil {
		return nil, err
	}
	// Convert values to pointers for backward compatibility
	result := make([]*dbindex.IndexInfo, len(infoList))
	for idx := range infoList {
		result[idx] = &infoList[idx]
	}
	return result, nil
}

func (i indexBackend) GetByPath(path string) (*dbindex.IndexInfo, error) {
	info, err := GetIndexInfo(path)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func (i indexBackend) GetBySource(source string) ([]*dbindex.IndexInfo, error) {
	infoList, err := GetIndexInfoBySource(source)
	if err != nil {
		return nil, err
	}
	// Convert values to pointers for backward compatibility
	result := make([]*dbindex.IndexInfo, len(infoList))
	for idx := range infoList {
		result[idx] = &infoList[idx]
	}
	return result, nil
}

func (i indexBackend) Save(info *dbindex.IndexInfo) error {
	// Check if exists and route to appropriate function
	indexMux.RLock()
	_, exists := indexInfoByPath[info.Path]
	indexMux.RUnlock()

	if exists {
		return UpdateIndexInfo(info)
	}
	return CreateIndexInfo(info)
}

func (i indexBackend) Delete(path string) error {
	return DeleteIndexInfo(path)
}
