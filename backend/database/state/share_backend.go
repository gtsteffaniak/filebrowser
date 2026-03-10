package state

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
)

// shareBackend implements share.StorageBackend using state
type shareBackend struct{}

func (s shareBackend) All() ([]*share.Link, error) {
	return GetAllShares()
}

func (s shareBackend) FindByUserID(id uint) ([]*share.Link, error) {
	return GetSharesByUserID(id)
}

func (s shareBackend) GetByHash(hash string) (*share.Link, error) {
	link, err := GetShare(hash)
	if err != nil && err.Error() == "share not found" {
		return nil, errors.ErrNotExist
	}
	return link, err
}

func (s shareBackend) GetCommonShareByHash(hash string) (*share.CommonShare, error) {
	link, err := GetShare(hash)
	if err != nil {
		if err.Error() == "share not found" {
			return nil, errors.ErrNotExist
		}
		return nil, err
	}
	cs := link.CommonShare
	cs.HasPassword = link.HasPassword()
	return &cs, nil
}

func (s shareBackend) GetPermanent(path, source string, id uint) (*share.Link, error) {
	link, err := sqlStore.GetPermanentShare(source, path, id)
	if err != nil {
		return nil, err
	}
	return link, nil
}

func (s shareBackend) GetBySourcePath(path, source string) ([]*share.Link, error) {
	links, err := GetSharesByPath(source, path)
	if err != nil {
		return nil, err
	}
	if links == nil {
		return []*share.Link{}, nil
	}
	return links, nil
}

func (s shareBackend) Gets(path, source string, id uint) ([]*share.Link, error) {
	links, err := GetSharesByPath(source, path)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	filtered := make([]*share.Link, 0)
	for _, l := range links {
		if l.UserID == id && (l.Expire == 0 || l.Expire > now || l.KeepAfterExpiration) {
			filtered = append(filtered, l)
		}
	}
	return filtered, nil
}

func (s shareBackend) Save(l *share.Link) error {
	return SaveShare(l)
}

func (s shareBackend) Delete(hash string) error {
	return DeleteShare(hash)
}
