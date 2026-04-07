package users

import (
	"fmt"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/crud"
)

// StorageBackend is the interface to implement for a users storage.
// All lookups and deletes use stable numeric user ids.
type StorageBackend interface {
	GetBy(id uint64) (*User, error)
	Gets() ([]*User, error)
	Save(u *User, changePass bool, disableScopeChange bool) error
	Update(u *User, adminActor bool, fields ...string) error
	DeleteByID(id uint64) error
}

// Store is an interface for user storage.
type Store interface {
	Get(id uint64) (user *User, err error)
	Gets() ([]*User, error)
	Update(user *User, adminActor bool, fields ...string) error
	Save(user *User, changePass bool, disableScopeChange bool) error
	Delete(id uint64) error
	LastUpdate(id uint64) int64
	AddApiToken(userID uint64, name string, tokenString string, metadata AuthToken) error
	DeleteApiToken(userID uint64, name string) error
}

// usernameToID is set from state (or tests) so packages like auth can resolve
// login names to ids without importing state (avoids import cycles).
var usernameToID func(string) (uint64, error)

// SetUsernameToID registers login name → stable id resolution. Call from state.Initialize after users load.
func SetUsernameToID(fn func(string) (uint64, error)) {
	usernameToID = fn
}

// ResolveUsernameToID maps a login name to stable user id using the resolver from SetUsernameToID.
func ResolveUsernameToID(username string) (uint64, error) {
	if usernameToID == nil {
		return 0, fmt.Errorf("users: login name resolver not configured")
	}
	return usernameToID(username)
}

// crudBackend implements crud.CrudBackend[User] for users storage.
type crudBackend struct {
	back StorageBackend
}

func (c *crudBackend) GetByID(id any) (*User, error) {
	switch v := id.(type) {
	case uint64:
		if v == 0 {
			return nil, errors.ErrNotExist
		}
		return c.back.GetBy(v)
	case uint:
		if v == 0 {
			return nil, errors.ErrNotExist
		}
		return c.back.GetBy(uint64(v))
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
	case uint64:
		return c.back.DeleteByID(v)
	case uint:
		return c.back.DeleteByID(uint64(v))
	default:
		return errors.ErrInvalidDataType
	}
}

// Storage is a users storage using generics.
type Storage struct {
	Generic *crud.Storage[User]
	back    StorageBackend
	updated map[uint64]int64
	mux     sync.RWMutex
}

// NewStorage creates a users storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{
		Generic: crud.NewStorage[User](&crudBackend{back: back}),
		back:    back,
		updated: map[uint64]int64{},
	}
}

// Get returns a user by stable numeric id.
func (s *Storage) Get(id uint64) (user *User, err error) {
	if id == 0 {
		return nil, errors.ErrNotExist
	}
	return s.back.GetBy(id)
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
	if user.ID != 0 {
		s.updated[user.ID] = time.Now().Unix()
	}
	s.mux.Unlock()
	return nil
}

func (s *Storage) AddApiToken(userID uint64, name string, tokenString string, metadata AuthToken) error {
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

func (s *Storage) DeleteApiToken(userID uint64, name string) error {
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

// Delete removes a user by stable numeric id.
func (s *Storage) Delete(id uint64) error {
	if id == 0 {
		return errors.ErrInvalidDataType
	}
	return s.back.DeleteByID(id)
}

// LastUpdate gets the timestamp for the last update of a user by id.
func (s *Storage) LastUpdate(id uint64) int64 {
	s.mux.RLock()
	defer s.mux.RUnlock()
	if val, ok := s.updated[id]; ok {
		return val
	}
	return 0
}
