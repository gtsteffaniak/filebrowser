package bolt

import (
	storm "github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

type indexingBackend struct {
	db *storm.DB
}

func (s indexingBackend) All() ([]*dbindex.IndexInfo, error) {
	var v []*dbindex.IndexInfo
	err := s.db.All(&v)
	if err == storm.ErrNotFound {
		return v, errors.ErrNotExist
	}

	return v, err
}

func (s indexingBackend) GetByPath(path string) (*dbindex.IndexInfo, error) {
	var v dbindex.IndexInfo
	err := s.db.One("Path", path, &v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}

	return &v, err
}

func (s indexingBackend) GetBySource(source string) ([]*dbindex.IndexInfo, error) {
	var v []*dbindex.IndexInfo
	err := s.db.Select(q.Eq("Source", source)).Find(&v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}
	return v, err
}

func (s indexingBackend) Save(info *dbindex.IndexInfo) error {
	return s.db.Save(info)
}

func (s indexingBackend) Delete(path string) error {
	err := s.db.DeleteStruct(&dbindex.IndexInfo{Path: path})
	if err == storm.ErrNotFound {
		return nil
	}
	return err
}

