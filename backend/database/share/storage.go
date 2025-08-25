package share

import (
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// StorageBackend is the interface to implement for a share storage.
type StorageBackend interface {
	All() ([]*Link, error)
	FindByUserID(id uint) ([]*Link, error)
	GetByHash(hash string) (*Link, error)
	GetPermanent(path, source string, id uint) (*Link, error)
	GetBySourcePath(path, source string) ([]*Link, error)
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
	Generic    *crud.Storage[Link]
	back       StorageBackend
	shareCache map[string]*Link
	mu         sync.RWMutex
	users      *users.Storage
}

// NewStorage creates a share links storage from a backend.
func NewStorage(back StorageBackend, usersStore *users.Storage) *Storage {
	return &Storage{
		Generic:    crud.NewStorage[Link](&crudBackend{back: back}),
		back:       back,
		shareCache: make(map[string]*Link),
		users:      usersStore,
	}
}

// All wraps StorageBackend.All and handles expiry.
func (s *Storage) All() ([]*Link, error) {
	links, err := s.back.All()
	if err != nil {
		return nil, err
	}
	filtered, err := s.filterExpired(links)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	for i, l := range filtered {
		if l == nil {
			continue
		}
		if existing, ok := s.shareCache[l.Hash]; ok && existing != nil {
			filtered[i] = existing
		} else {
			s.shareCache[l.Hash] = l
		}
	}
	s.mu.Unlock()
	return filtered, nil
}

// FindByUserID wraps StorageBackend.FindByUserID and handles expiry.
func (s *Storage) FindByUserID(id uint) ([]*Link, error) {
	links, err := s.back.FindByUserID(id)
	if err != nil {
		return nil, err
	}
	filtered, err := s.filterExpired(links)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	for i, l := range filtered {
		if l == nil {
			continue
		}
		if existing, ok := s.shareCache[l.Hash]; ok && existing != nil {
			filtered[i] = existing
		} else {
			s.shareCache[l.Hash] = l
		}
	}
	s.mu.Unlock()
	return filtered, nil
}

// GetByHash wraps StorageBackend.GetByHash and handles expiry.
func (s *Storage) GetByHash(hash string) (*Link, error) {
	// return stable in-memory pointer if available
	s.mu.RLock()
	if link, ok := s.shareCache[hash]; ok && link != nil {
		s.mu.RUnlock()
		if link.Expire != 0 && link.Expire <= time.Now().Unix() {
			_ = s.back.Delete(hash)
			s.mu.Lock()
			delete(s.shareCache, hash)
			s.mu.Unlock()
			return nil, errors.ErrNotExist
		}
		return link, nil
	}
	s.mu.RUnlock()

	link, err := s.back.GetByHash(hash)
	if err != nil {
		return nil, err
	}
	if link.Expire != 0 && link.Expire <= time.Now().Unix() {
		_ = s.back.Delete(hash)
		return nil, errors.ErrNotExist
	}
	s.mu.Lock()

	s.shareCache[hash] = link
	s.mu.Unlock()
	return link, nil
}

// GetPermanent wraps StorageBackend.GetPermanent
func (s *Storage) GetPermanent(path, source string, id uint) (*Link, error) {
	l, err := s.back.GetPermanent(path, source, id)
	if err == nil && l != nil {
		s.mu.Lock()

		s.shareCache[l.Hash] = l
		s.mu.Unlock()
	}
	return l, err
}

// Gets wraps StorageBackend.Gets and handles expiry.
func (s *Storage) Gets(sourcePath, source string, id uint) ([]*Link, error) {
	links, err := s.back.Gets(sourcePath, source, id)
	if err != nil {
		return nil, err
	}
	filtered, err := s.filterExpired(links)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	for i, l := range filtered {
		if l == nil {
			continue
		}
		if existing, ok := s.shareCache[l.Hash]; ok && existing != nil {
			filtered[i] = existing
		} else {
			s.shareCache[l.Hash] = l
		}
	}
	s.mu.Unlock()
	return filtered, nil
}

// GetBySourcePath wraps StorageBackend.GetBySourcePath and handles expiry.
func (s *Storage) GetBySourcePath(path, source string) ([]*Link, error) {
	links, err := s.back.GetBySourcePath(path, source)
	if err != nil {
		return nil, err
	}
	filtered, err := s.filterExpired(links)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	for i, l := range filtered {
		if l == nil {
			continue
		}
		if existing, ok := s.shareCache[l.Hash]; ok && existing != nil {
			filtered[i] = existing
		} else {
			s.shareCache[l.Hash] = l
		}
	}
	s.mu.Unlock()
	return filtered, nil
}

// UpdateShares updates all shares that exactly match oldSource and oldPath
// to point to newSource and newPath. Returns the number of updated shares.
func (s *Storage) UpdateShares(oldSource, oldPath, newSource, newPath string) (int, error) {
	links, err := s.back.All()
	if err != nil && err != errors.ErrNotExist {
		logger.Error("failed to list shares", "error", err)
		return 0, err
	}
	updated := 0
	for _, l := range links {
		if l == nil {
			continue
		}
		if l.Source != oldSource {
			continue
		}
		pos := strings.Index(l.Path, oldPath)
		if pos < 0 {
			continue
		}
		oldFullPath := l.Path
		l.Source = newSource
		l.Path = oldFullPath[:pos] + newPath + oldFullPath[pos+len(oldPath):]
		if err := s.back.Save(l); err != nil {
			logger.Error("failed to save updated share", "hash", l.Hash, "error", err)
			return updated, err
		}
		s.mu.Lock()
		if existing, ok := s.shareCache[l.Hash]; ok && existing != nil {
			// update existing cached pointer fields
			existing.Source = l.Source
			existing.Path = l.Path
		} else {
			s.shareCache[l.Hash] = l
		}
		s.mu.Unlock()
		logger.Info("share updated", "hash", l.Hash, "fromPath", oldFullPath, "toPath", l.Path)
		updated++
	}
	if updated == 0 {
		logger.Warning("no matching shares to update for provided source/path")
	}
	return updated, nil
}

// Save wraps StorageBackend.Save
func (s *Storage) Save(l *Link) error {
	if err := s.back.Save(l); err != nil {
		return err
	}
	s.mu.Lock()
	s.shareCache[l.Hash] = l
	s.mu.Unlock()
	return nil
}

// Delete wraps StorageBackend.Delete
func (s *Storage) Delete(hash string) error {
	if err := s.back.Delete(hash); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.shareCache, hash)
	s.mu.Unlock()
	return nil
}

// Flush persists the current in-memory state of all shares to the backing store.
// Call during graceful shutdown to ensure DB matches memory.
func (s *Storage) Flush() error {
	s.mu.RLock()
	links := make([]*Link, 0, len(s.shareCache))
	for _, l := range s.shareCache {
		if l != nil {
			links = append(links, l)
		}
	}
	s.mu.RUnlock()
	for _, l := range links {
		if err := s.back.Save(l); err != nil {
			return err
		}
	}
	return nil
}

// filterExpired removes expired links and deletes them from storage.
func (s *Storage) filterExpired(links []*Link) ([]*Link, error) {
	var filtered []*Link
	for _, link := range links {
		if link.Expire != 0 && link.Expire <= time.Now().Unix() && !link.KeepAfterExpiration {
			_ = s.back.Delete(link.Hash)
			s.mu.Lock()
			delete(s.shareCache, link.Hash)
			s.mu.Unlock()
			continue
		}
		filtered = append(filtered, link)
	}
	return filtered, nil
}
