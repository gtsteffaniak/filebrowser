package auth

import (
	"encoding/base64"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
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
func NewStorage(back StorageBackend, userStore *users.Storage) (*Storage, error) {
	store := &Storage{back: back, users: userStore}
	err := store.Save(&JSONAuth{})
	if err != nil {
		return nil, err
	}
	err = store.Save(&ProxyAuth{})
	if err != nil {
		return nil, err
	}
	err = store.Save(&HookAuth{})
	if err != nil {
		return nil, err
	}
	err = store.Save(&NoAuth{})
	if err != nil {
		return nil, err
	}
	encryptionKey, err = base64.StdEncoding.DecodeString(settings.Config.Auth.TotpSecret)
	if err != nil {
		logger.Warning("failed to decode TOTP secret, using default key. This is insecure and should be fixed.")
	}
	return store, nil
}

// Get wraps a StorageBackend.Get.
func (s *Storage) Get(t string) (Auther, error) {
	return s.back.Get(t)
}

// Save wraps a StorageBackend.Save.
func (s *Storage) Save(a Auther) error {
	return s.back.Save(a)
}
