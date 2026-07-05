package indexing

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/ports"
)

var metaStore ports.IndexMetaStore

// SetMetaStore registers the index metadata persistence port (called from app.WireServices).
func SetMetaStore(s ports.IndexMetaStore) {
	metaStore = s
}

func getIndexInfoByPath(path string) (*dbindex.IndexInfo, error) {
	if metaStore == nil {
		return nil, errors.ErrNotExist
	}
	info, err := metaStore.GetIndexInfo(path)
	if err != nil {
		return nil, err
	}
	return &info, nil
}

func saveIndexInfo(info *dbindex.IndexInfo) error {
	if metaStore == nil {
		return nil
	}
	return metaStore.SaveIndexInfo(info)
}

func metaStoreConfigured() bool {
	return metaStore != nil
}
