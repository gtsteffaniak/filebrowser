package share

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// StorageBackend is the interface to implement for a share storage.
type StorageBackend interface {
	All() ([]*Share, error)
	FindByUserID(userID uint64) ([]*Share, error)
	GetByHash(hash string) (*Share, error)
	GetShareInfoByHash(hash string) (*FrontendShareInfo, error)
	GetPermanent(path, source string, userID uint64) (*Share, error)
	GetBySourcePath(path, source string) ([]*Share, error)
	Gets(path, source string, userID uint64) ([]*Share, error)
	Save(s *Share) error
	Delete(hash string) error
}

// crudBackend implements crud.CrudBackend[ShareInfo] for share storage.
type crudBackend struct {
	back StorageBackend
}

func (c *crudBackend) GetByID(id any) (*Share, error) {
	hash, ok := id.(string)
	if !ok {
		return nil, errors.ErrInvalidDataType
	}
	return c.back.GetByHash(hash)
}

func (c *crudBackend) GetAll() ([]*Share, error) {
	return c.back.All()
}

func (c *crudBackend) Save(obj *Share) error {
	return c.back.Save(obj)
}

func (c *crudBackend) DeleteByID(id any) error {
	hash, ok := id.(string)
	if !ok {
		return errors.ErrInvalidDataType
	}
	return c.back.Delete(hash)
}

// Storage fronts StorageBackend (e.g. state.shareBackend). Authoritative in-memory shares live in
// package state, updated after successful database writes; this wrapper does not keep a second copy.
type Storage struct {
	Generic *crud.Storage[Share]
	back    StorageBackend
	users   *users.Storage
}

// NewStorage returns Storage that delegates all operations to back.
func NewStorage(back StorageBackend, usersStore *users.Storage) *Storage {
	return &Storage{
		Generic: crud.NewStorage[Share](&crudBackend{back: back}),
		back:    back,
		users:   usersStore,
	}
}

// All returns all non-expired shares from the backend.
func (s *Storage) All() ([]*Share, error) {
	return s.back.All()
}

// FindByUserID returns non-expired shares owned by userID from the backend.
func (s *Storage) FindByUserID(userID uint64) ([]*Share, error) {
	return s.back.FindByUserID(userID)
}

// PrepForFrontend returns API-safe ShareFrontend copies for one or more shares.
func (s *Storage) PrepForFrontend(viewer *users.User, r *http.Request, links ...*Share) []*ShareFrontend {
	return PrepForFrontend(viewer, s.users, r, links...)
}

// GetByHash wraps StorageBackend.GetByHash and handles expiry.
func (s *Storage) GetByHash(hash string) (*Share, error) {
	link, err := s.back.GetByHash(hash)
	if err != nil {
		return nil, err
	}
	if link.Expire != 0 && link.Expire <= time.Now().Unix() {
		_ = s.back.Delete(hash)
		return nil, errors.ErrNotExist
	}
	link.InitUserDownloads()
	return link, nil
}

// GetPermanent wraps StorageBackend.GetPermanent
func (s *Storage) GetPermanent(path, source string, userID uint64) (*Share, error) {
	return s.back.GetPermanent(path, source, userID)
}

// Gets returns shares for the given path, source, and owner user id.
func (s *Storage) Gets(sourcePath, source string, userID uint64) ([]*Share, error) {
	return s.back.Gets(sourcePath, source, userID)
}

// GetBySourcePath returns shares for the given path and source.
func (s *Storage) GetBySourcePath(path, source string) ([]*Share, error) {
	return s.back.GetBySourcePath(path, source)
}

// IsShared returns whether the given path and source have any shares for owner userID.
func (s *Storage) IsShared(path, source string, userID uint64) bool {
	links, _ := s.GetBySourcePath(path, source)
	for _, l := range links {
		if l.UserID == userID {
			return true
		}
	}
	return len(links) > 0
}

// UpdateShares updates all shares that match oldSource and oldPath to point to newSource and newPath.
// Handles both exact matches and subdirectories, regardless of trailing slashes.
func (s *Storage) UpdateShares(oldSource, oldPath, newSource, newPath string) (int, error) {
	links, err := s.back.All()
	if err != nil && err != errors.ErrNotExist {
		logger.Error("failed to list shares", "error", err)
		return 0, err
	}

	oldPath = utils.AddTrailingSlashIfNotExists(oldPath)
	newPath = utils.AddTrailingSlashIfNotExists(newPath)

	updated := 0
	for _, l := range links {
		if l == nil || l.SourcePath != oldSource {
			continue
		}
		l.Path = utils.AddTrailingSlashIfNotExists(l.Path)

		pos := strings.Index(l.Path, oldPath)
		if pos < 0 {
			continue
		}

		l.SourcePath = newSource
		l.Path = newPath

		if err := s.back.Save(l); err != nil {
			logger.Error("failed to save updated share", "hash", l.Hash, "error", err)
			return updated, err
		}
		updated++
	}
	return updated, nil
}

// UpdateSharePath updates the path for a specific share identified by hash
func (s *Storage) UpdateSharePath(hash, newPath string) error {
	link, err := s.GetByHash(hash)
	if err != nil {
		return err
	}

	link.Path = newPath

	if err := s.back.Save(link); err != nil {
		logger.Error("failed to save updated share", "hash", hash, "error", err)
		return err
	}

	logger.Debug("share path updated", "hash", hash, "toPath", newPath)
	return nil
}

// CreateShare creates a new share via the backend (database + state).
func (s *Storage) CreateShare(l *Share) error {
	_, err := s.back.GetByHash(l.Hash)
	if err == nil {
		return fmt.Errorf("share with hash %s already exists", l.Hash)
	}
	if !stderrors.Is(err, errors.ErrNotExist) {
		return err
	}
	return s.back.Save(l)
}

// UpdateShare updates an existing share via the backend.
func (s *Storage) UpdateShare(l *Share) error {
	if _, err := s.back.GetByHash(l.Hash); err != nil {
		if stderrors.Is(err, errors.ErrNotExist) {
			return fmt.Errorf("share with hash %s not found", l.Hash)
		}
		return err
	}
	return s.back.Save(l)
}

// Delete wraps StorageBackend.Delete
func (s *Storage) Delete(hash string) error {
	return s.back.Delete(hash)
}

// Flush is a no-op: writes go through the backend immediately.
func (s *Storage) Flush() error {
	return nil
}

// GetShareInfoByHash returns share presentation fields for a hash (e.g. public/info).
func (s *Storage) GetShareInfoByHash(hash string) (*FrontendShareInfo, error) {
	return s.back.GetShareInfoByHash(hash)
}
