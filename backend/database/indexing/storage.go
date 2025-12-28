package indexing

import (
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
)

// StorageBackend is the interface to implement for an indexing storage.
type StorageBackend interface {
	All() ([]*IndexInfo, error)
	GetByPath(path string) (*IndexInfo, error)
	GetBySource(source string) ([]*IndexInfo, error)
	Save(info *IndexInfo) error
	Delete(path string) error
}

// crudBackend implements crud.CrudBackend[IndexInfo] for indexing storage.
type crudBackend struct {
	back StorageBackend
}

func (c *crudBackend) GetByID(id any) (*IndexInfo, error) {
	path, ok := id.(string)
	if !ok {
		return nil, errors.ErrInvalidDataType
	}
	return c.back.GetByPath(path)
}

func (c *crudBackend) GetAll() ([]*IndexInfo, error) {
	return c.back.All()
}

func (c *crudBackend) Save(obj *IndexInfo) error {
	return c.back.Save(obj)
}

func (c *crudBackend) DeleteByID(id any) error {
	path, ok := id.(string)
	if !ok {
		return errors.ErrInvalidDataType
	}
	return c.back.Delete(path)
}

// Storage is an indexing storage using generics.
type Storage struct {
	Generic      *crud.Storage[IndexInfo]
	back         StorageBackend
	indexCache   map[string]*IndexInfo
	mu           sync.RWMutex
}

// NewStorage creates an indexing storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{
		Generic:    crud.NewStorage[IndexInfo](&crudBackend{back: back}),
		back:       back,
		indexCache: make(map[string]*IndexInfo),
	}
}

// All wraps StorageBackend.All
func (s *Storage) All() ([]*IndexInfo, error) {
	infos, err := s.back.All()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	for i, info := range infos {
		if info == nil {
			continue
		}
		if existing, ok := s.indexCache[info.Path]; ok && existing != nil {
			infos[i] = existing
		} else {
			s.indexCache[info.Path] = info
		}
	}
	s.mu.Unlock()
	return infos, nil
}

// GetByPath wraps StorageBackend.GetByPath
func (s *Storage) GetByPath(path string) (*IndexInfo, error) {
	// return stable in-memory pointer if available
	s.mu.RLock()
	if info, ok := s.indexCache[path]; ok && info != nil {
		s.mu.RUnlock()
		return info, nil
	}
	s.mu.RUnlock()

	info, err := s.back.GetByPath(path)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.indexCache[path] = info
	s.mu.Unlock()
	return info, nil
}

// GetBySource wraps StorageBackend.GetBySource
func (s *Storage) GetBySource(source string) ([]*IndexInfo, error) {
	infos, err := s.back.GetBySource(source)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	for i, info := range infos {
		if info == nil {
			continue
		}
		if existing, ok := s.indexCache[info.Path]; ok && existing != nil {
			infos[i] = existing
		} else {
			s.indexCache[info.Path] = info
		}
	}
	s.mu.Unlock()
	return infos, nil
}

// Save wraps StorageBackend.Save
func (s *Storage) Save(info *IndexInfo) error {
	if err := s.back.Save(info); err != nil {
		return err
	}
	s.mu.Lock()
	s.indexCache[info.Path] = info
	s.mu.Unlock()
	return nil
}

// Delete wraps StorageBackend.Delete
func (s *Storage) Delete(path string) error {
	if err := s.back.Delete(path); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.indexCache, path)
	s.mu.Unlock()
	return nil
}

// Flush persists the current in-memory state of all indexes to the backing store.
// Call during graceful shutdown to ensure DB matches memory.
func (s *Storage) Flush() error {
	s.mu.RLock()
	infos := make([]*IndexInfo, 0, len(s.indexCache))
	for _, info := range s.indexCache {
		if info != nil {
			infos = append(infos, info)
		}
	}
	s.mu.RUnlock()
	for _, info := range infos {
		if err := s.back.Save(info); err != nil {
			return err
		}
	}
	return nil
}

