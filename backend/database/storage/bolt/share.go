package bolt

import (
	"time"

	storm "github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/go-logger/logger"
)

type shareBackend struct {
	db *storm.DB
}

func (s shareBackend) All() ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.All(&v)
	if err == storm.ErrNotFound {
		return v, errors.ErrNotExist
	}

	return v, err
}

func (s shareBackend) FindByUserID(id uint) ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.Select(q.Eq("UserID", id)).Find(&v)
	if err == storm.ErrNotFound {
		return v, errors.ErrNotExist
	}

	return v, err
}

func (s shareBackend) GetByHash(hash string) (*share.Link, error) {
	var v share.Link
	err := s.db.One("Hash", hash, &v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}

	return &v, err
}

func (s shareBackend) GetCommonShareByHash(hash string) (*share.CommonShare, error) {
	var v share.Link
	err := s.db.One("Hash", hash, &v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	v.Source = ""
	v.Path = ""
	v.AllowedUsernames = nil
	v.DownloadsLimit = 0
	return &v.CommonShare, nil
}

func (s shareBackend) GetPermanent(path, source string, id uint) (*share.Link, error) {
	var v share.Link
	// TODO remove legacy and return notfound errors
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", source), q.Eq("Expire", 0), q.Eq("UserID", id)).First(&v)
	return &v, nil
}

// GetBySourcePath returns all shares that exactly match Path and Source across users.
func (s shareBackend) GetBySourcePath(path, source string) ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.Select(q.Eq("Path", path), q.Eq("Source", source)).Find(&v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}
	return v, err
}

func (s shareBackend) Gets(path, sourcePath string, id uint) ([]*share.Link, error) {
	var v []*share.Link
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", sourcePath), q.Eq("UserID", id)).Find(&v)
	filteredList := []*share.Link{}
	var err error
	// through and filter out expired share
	for i := range v {
		if v[i].Expire < time.Now().Unix() && v[i].Expire != 0 {
			err = s.Delete(v[i].Hash)
			if err != nil {
				logger.Errorf("expired share could not be deleted: %v", err.Error())
			}
		} else {
			filteredList = append(filteredList, v[i])
		}
	}
	return filteredList, err
}

func (s shareBackend) Save(l *share.Link) error {
	return s.db.Save(l)
}

func (s shareBackend) Delete(hash string) error {
	err := s.db.DeleteStruct(&share.Link{Hash: hash})
	if err == storm.ErrNotFound {
		return nil
	}
	return err
}
