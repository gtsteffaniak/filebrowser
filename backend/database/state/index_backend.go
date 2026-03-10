package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

// indexBackend implements dbindex.StorageBackend using state
type indexBackend struct{}

func (i indexBackend) All() ([]*dbindex.IndexInfo, error) {
	return GetAllIndexInfo()
}

func (i indexBackend) GetByPath(path string) (*dbindex.IndexInfo, error) {
	info, err := GetIndexInfo(path)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (i indexBackend) GetBySource(source string) ([]*dbindex.IndexInfo, error) {
	return GetIndexInfoBySource(source)
}

func (i indexBackend) Save(info *dbindex.IndexInfo) error {
	return SaveIndexInfo(info)
}

func (i indexBackend) Delete(path string) error {
	return DeleteIndexInfo(path)
}
