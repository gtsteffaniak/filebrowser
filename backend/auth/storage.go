package auth

import (
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

// StorageBackend is a storage backend for auth storage.
type StorageBackend interface {
	Get(string) (Auther, error)
	Save(Auther) error
}

// Storage is a auth storage.
type Storage struct {
	back  StorageBackend
	users *users.Storage
}

// NewStorage creates a auth storage from a backend.
func NewStorage(back StorageBackend, userStore *users.Storage) *Storage {
	return &Storage{back: back, users: userStore}
}

// Get wraps a StorageBackend.Get.
func (s *Storage) Get(t string) (Auther, error) {
	return s.back.Get(t)
}

// Save wraps a StorageBackend.Save.
func (s *Storage) Save(a Auther) error {
	return s.back.Save(a)
}
