package bolt

import (
	"fmt"
	"time"

	"github.com/asdine/storm/v3"
	"github.com/asdine/storm/v3/q"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
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

func (s shareBackend) GetPermanent(path string, id uint) (*share.Link, error) {
	var v share.Link
	err := s.db.Select(q.Eq("Path", path), q.Eq("Expire", 0), q.Eq("UserID", id)).First(&v)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}

	return &v, err
}

func (s shareBackend) Gets(path string, id uint) ([]*share.Link, error) {
	var v []*share.Link
	err := s.db.Select(q.Eq("Path", path), q.Eq("UserID", id)).Find(&v)
	if err == storm.ErrNotFound {
		return v, errors.ErrNotExist
	}

	filteredList := []*share.Link{}
	// automatically delete and clear expired shares
	for i := range v {
		if v[i].Expire < time.Now().Unix() {
			err = s.Delete(v[i].PasswordHash)
			if err != nil {
				fmt.Println("expired share could not be deleted: ", err.Error())
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
