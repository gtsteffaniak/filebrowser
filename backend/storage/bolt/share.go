package bolt

import (
	"fmt"
	"time"

	storm "github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/share"
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

func (s shareBackend) GetPermanent(path, source string, id uint) (*share.Link, error) {
	var v share.Link
	var legacy []*share.Link
	// todo remove legacy and return notfound errors
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", source), q.Eq("Expire", 0), q.Eq("UserID", id)).First(&v)
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", ""), q.Eq("UserID", id)).Find(&legacy)
	return &v, nil
}

func (s shareBackend) Gets(path, source string, id uint) ([]*share.Link, error) {
	var v []*share.Link
	var legacy []*share.Link
	// todo remove legacy and return notfound errors
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", source), q.Eq("UserID", id)).Find(&v)
	_ = s.db.Select(q.Eq("Path", path), q.Eq("Source", ""), q.Eq("UserID", id)).Find(&legacy)
	filteredList := []*share.Link{}
	var err error
	// through and filter out expired share
	for i := range v {
		if v[i].Expire < time.Now().Unix() && v[i].Expire != 0 {
			fmt.Println("deleting")

			err = s.Delete(v[i].PasswordHash)
			if err != nil {
				logger.Error(fmt.Sprintf("expired share could not be deleted: %v", err.Error()))
			}
		} else {
			filteredList = append(filteredList, v[i])
		}
	}

	// automatically delete and clear expired shares
	// todo remove after some time.
	for i := range legacy {
		if legacy[i].Expire < time.Now().Unix() && legacy[i].Expire != 0 {
			fmt.Println("deleting")
			err = s.Delete(legacy[i].PasswordHash)
			if err != nil {
				logger.Error(fmt.Sprintf("expired share could not be deleted: %v", err.Error()))
			}
		} else {
			filteredList = append(filteredList, legacy[i])
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
