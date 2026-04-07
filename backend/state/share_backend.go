package state

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/go-logger/logger"
)

// shareBackend implements share.StorageBackend using state
type shareBackend struct{}

func (s shareBackend) All() ([]*share.Link, error) {
	sharesList, err := GetAllShares()
	if err != nil {
		return nil, err
	}
	// Convert values to pointers for backward compatibility
	result := make([]*share.Link, len(sharesList))
	for i := range sharesList {
		result[i] = &sharesList[i]
	}
	return result, nil
}

func (s shareBackend) FindByUserID(userID uint64) ([]*share.Link, error) {
	sharesList, err := GetSharesByUserID(userID)
	if err != nil {
		return nil, err
	}
	result := make([]*share.Link, len(sharesList))
	for i := range sharesList {
		result[i] = &sharesList[i]
	}
	return result, nil
}

func (s shareBackend) GetByHash(hash string) (*share.Link, error) {
	link, err := GetShare(hash)
	if err != nil && err.Error() == "share not found" {
		return nil, errors.ErrNotExist
	}
	if err != nil {
		return nil, err
	}
	return &link, nil
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

func (s shareBackend) GetPermanent(path, source string, userID uint64) (*share.Link, error) {
	link, err := sqlStore.GetPermanentShare(source, path, userID)
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
	if len(links) == 0 {
		return []*share.Link{}, nil
	}
	// Convert values to pointers for backward compatibility
	result := make([]*share.Link, len(links))
	for i := range links {
		result[i] = &links[i]
	}
	return result, nil
}

func (s shareBackend) Gets(path, source string, userID uint64) ([]*share.Link, error) {
	logger.Debug("shareBackend.Gets ENTRY", "path", path, "source", source, "userID", userID)
	links, err := GetSharesByPath(source, path)
	logger.Debug("shareBackend.Gets after GetSharesByPath", "count", len(links), "err", err)
	if err != nil {
		return nil, err
	}
	now := time.Now().Unix()
	filtered := make([]*share.Link, 0)
	for i := range links {
		l := &links[i]
		if l.UserID == userID && (l.Expire == 0 || l.Expire > now || l.KeepAfterExpiration) {
			filtered = append(filtered, l)
		}
	}
	logger.Debug("shareBackend.Gets filtered", "filtered", len(filtered))
	return filtered, nil
}

func (s shareBackend) Save(l *share.Link) error {
	// Check if share exists
	_, err := GetShare(l.Hash)
	if err != nil {
		// Share doesn't exist, create it
		return CreateShare(l)
	}
	// Share exists, update it
	return UpdateShare(l.Hash, func(existingShare *share.Link) error {
		// Copy all fields from l to existingShare
		*existingShare = *l
		return nil
	})
}

func (s shareBackend) Delete(hash string) error {
	return DeleteShare(hash)
}
