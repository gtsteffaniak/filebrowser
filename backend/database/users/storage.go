package users

import (
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
)

// StorageBackend is the interface to implement for a users storage.
type StorageBackend interface {
	GetBy(interface{}) (*User, error)
	Gets() ([]*User, error)
	Save(u *User, changePass bool, disableScopeChange bool) error
	Update(u *User, adminActor bool, fields ...string) error
	DeleteByID(uint) error
	DeleteByUsername(string) error
}

// Store is an interface for user storage.
type Store interface {
	Get(id interface{}) (user *User, err error)
	Gets() ([]*User, error)
	Update(user *User, adminActor bool, fields ...string) error
	Save(user *User, changePass bool, disableScopeChange bool) error
	Delete(id interface{}) error
	LastUpdate(id uint) int64
	AddApiToken(userID uint, name string, tokenString string, metadata AuthToken) error
	DeleteApiToken(userID uint, name string) error
}

// crudBackend implements crud.CrudBackend[User] for users storage.
type crudBackend struct {
	back StorageBackend
}

func (c *crudBackend) GetByID(id any) (*User, error) {
	switch v := id.(type) {
	case string, uint:
		return c.back.GetBy(v)
	default:
		return nil, errors.ErrInvalidDataType
	}
}

func (c *crudBackend) GetAll() ([]*User, error) {
	return c.back.Gets()
}

func (c *crudBackend) Save(obj *User) error {
	// Use default values for changePass and disableScopeChange
	return c.back.Save(obj, false, false)
}

func (c *crudBackend) DeleteByID(id any) error {
	switch v := id.(type) {
	case string:
		return c.back.DeleteByUsername(v)
	case uint:
		return c.back.DeleteByID(v)
	default:
		return errors.ErrInvalidDataType
	}
}

// Storage is a users storage using generics.
type Storage struct {
	Generic *crud.Storage[User]
	back    StorageBackend
	updated map[uint]int64
	mux     sync.RWMutex
}

// NewStorage creates a users storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{
		Generic: crud.NewStorage[User](&crudBackend{back: back}),
		back:    back,
		updated: map[uint]int64{},
	}
}

// Get allows you to get a user by its name or username. The provided
// id must be a string for username lookup or a uint for id lookup. If id
// is neither, a ErrInvalidDataType will be returned.
func (s *Storage) Get(id interface{}) (user *User, err error) {
	user, err = s.back.GetBy(id)
	if err != nil {
		return
	}
	return user, err
}

// Gets gets a list of all users.
func (s *Storage) Gets() ([]*User, error) {
	users, err := s.back.Gets()
	if err != nil {
		return nil, err
	}
	return users, err
}

// Update updates a user in the database.
func (s *Storage) Update(user *User, adminIActor bool, fields ...string) error {
	err := s.back.Update(user, adminIActor, fields...)
	if err != nil {
		return err
	}

	s.mux.Lock()
	s.updated[user.ID] = time.Now().Unix()
	s.mux.Unlock()
	return nil
}

func (s *Storage) AddApiToken(userID uint, name string, tokenString string, metadata AuthToken) error {
	user, err := s.Get(userID)
	if err != nil {
		return err
	}
	// Initialize the TokenHashes map if it is nil
	if user.Tokens == nil {
		user.Tokens = make(map[string]AuthToken)
	}
	metadata.Token = tokenString
	user.Tokens[name] = metadata
	err = s.Update(user, true, "Tokens")
	if err != nil {
		return err
	}

	return nil
}

func (s *Storage) DeleteApiToken(userID uint, name string) error {
	user, err := s.Get(userID)
	if err != nil {
		return err
	}
	// Initialize the Tokens map if it is nil
	if user.Tokens == nil {
		user.Tokens = make(map[string]AuthToken)
	}
	delete(user.Tokens, name)
	err = s.Update(user, true, "Tokens")
	if err != nil {
		return err
	}

	return nil
}

// Save saves the user in a storage.
func (s *Storage) Save(user *User, changePass, disableScopeChange bool) error {
	return s.back.Save(user, changePass, disableScopeChange)
}

// Delete allows you to delete a user by its name or username. The provided
// id must be a string for username lookup or a uint for id lookup. If id
// is neither, a ErrInvalidDataType will be returned.
func (s *Storage) Delete(id interface{}) error {
	switch id := id.(type) {
	case string:
		return s.back.DeleteByUsername(id)
	case uint:
		return s.back.DeleteByID(id)
	default:
		return errors.ErrInvalidDataType
	}
}

// LastUpdate gets the timestamp for the last update of an user.
func (s *Storage) LastUpdate(id uint) int64 {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if val, ok := s.updated[id]; ok {
		return val
	}
	return 0
}
