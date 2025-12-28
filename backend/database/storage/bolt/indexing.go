package bolt

import (
	storm "github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/indexing"
)

type indexingBackend struct {
	db *storm.DB
}

func (s indexingBackend) All() ([]*indexing.IndexInfo, error) {
	var v []*indexing.IndexInfo
	err := s.db.All(&v)
	if err == storm.ErrNotFound {
		return v, errors.ErrNotExist
	}

	return v, err
}

func (s indexingBackend) GetByPath(path string) (*indexing.IndexInfo, error) {
	var v indexing.IndexInfo
	err := s.db.One("Path", path, &v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}

	return &v, err
}

func (s indexingBackend) GetBySource(source string) ([]*indexing.IndexInfo, error) {
	var v []*indexing.IndexInfo
	err := s.db.Select(q.Eq("Source", source)).Find(&v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}
	return v, err
}

func (s indexingBackend) Save(info *indexing.IndexInfo) error {
	return s.db.Save(info)
}

func (s indexingBackend) Delete(path string) error {
	err := s.db.DeleteStruct(&indexing.IndexInfo{Path: path})
	if err == storm.ErrNotFound {
		return nil
	}
	return err
}

