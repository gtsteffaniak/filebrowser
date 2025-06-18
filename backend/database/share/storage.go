package share

import (
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
)

// StorageBackend is the interface to implement for a share storage.
type StorageBackend interface {
	All() ([]*Link, error)
	FindByUserID(id uint) ([]*Link, error)
	GetByHash(hash string) (*Link, error)
	GetPermanent(path, source string, id uint) (*Link, error)
	Gets(path, source string, id uint) ([]*Link, error)
	Save(s *Link) error
	Delete(hash string) error
}

// crudBackend implements crud.CrudBackend[Link] for share storage.
type crudBackend struct {
	back StorageBackend
}

func (c *crudBackend) GetByID(id any) (*Link, error) {
	hash, ok := id.(string)
	if !ok {
		return nil, errors.ErrInvalidDataType
	}
	return c.back.GetByHash(hash)
}

func (c *crudBackend) GetAll() ([]*Link, error) {
	return c.back.All()
}

func (c *crudBackend) Save(obj *Link) error {
	return c.back.Save(obj)
}

func (c *crudBackend) DeleteByID(id any) error {
	hash, ok := id.(string)
	if !ok {
		return errors.ErrInvalidDataType
	}
	return c.back.Delete(hash)
}

// Storage is a share storage using generics.
type Storage struct {
	Generic *crud.Storage[Link]
	back    StorageBackend
}

// NewStorage creates a share links storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{
		Generic: crud.NewStorage[Link](&crudBackend{back: back}),
		back:    back,
	}
}

// All wraps StorageBackend.All and handles expiry.
func (s *Storage) All() ([]*Link, error) {
	links, err := s.back.All()
	if err != nil {
		return nil, err
	}
	return s.filterExpired(links)
}

// FindByUserID wraps StorageBackend.FindByUserID and handles expiry.
func (s *Storage) FindByUserID(id uint) ([]*Link, error) {
	links, err := s.back.FindByUserID(id)
	if err != nil {
		return nil, err
	}
	return s.filterExpired(links)
}

// GetByHash wraps StorageBackend.GetByHash and handles expiry.
func (s *Storage) GetByHash(hash string) (*Link, error) {
	link, err := s.back.GetByHash(hash)
	if err != nil {
		return nil, err
	}
	if link.Expire != 0 && link.Expire <= time.Now().Unix() {
		_ = s.back.Delete(hash)
		return nil, errors.ErrNotExist
	}
	return link, nil
}

// GetPermanent wraps StorageBackend.GetPermanent
func (s *Storage) GetPermanent(path, source string, id uint) (*Link, error) {
	return s.back.GetPermanent(path, source, id)
}

// Gets wraps StorageBackend.Gets and handles expiry.
func (s *Storage) Gets(sourcePath, source string, id uint) ([]*Link, error) {
	links, err := s.back.Gets(sourcePath, source, id)
	if err != nil {
		return nil, err
	}
	return s.filterExpired(links)
}

// Save wraps StorageBackend.Save
func (s *Storage) Save(l *Link) error {
	return s.back.Save(l)
}

// Delete wraps StorageBackend.Delete
func (s *Storage) Delete(hash string) error {
	return s.back.Delete(hash)
}

// filterExpired removes expired links and deletes them from storage.
func (s *Storage) filterExpired(links []*Link) ([]*Link, error) {
	var filtered []*Link
	for _, link := range links {
		if link.Expire != 0 && link.Expire <= time.Now().Unix() {
			_ = s.back.Delete(link.Hash)
			continue
		}
		filtered = append(filtered, link)
	}
	return filtered, nil
}
