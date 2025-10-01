package auth

import (
	"encoding/base64"

	"crypto/sha256"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	"golang.org/x/crypto/pbkdf2"
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
	err = store.Save(&NoAuth{})
	if err != nil {
		return nil, err
	}
	if settings.Config.Auth.TotpSecret != "" {
		key, err := base64.StdEncoding.DecodeString(settings.Config.Auth.TotpSecret)
		keyLen := len(key)

		if err == nil && (keyLen == 16 || keyLen == 24 || keyLen == 32) {
			// Use the user-provided key if it's valid
			encryptionKey = key
		} else {
			// Otherwise, derive the key from the provided secret string
			logger.Warningf("totpSecret is not a valid Base64 encoded key. Deriving a key from it. For better security, generate a secret with 'openssl rand -base64 32'.")
			salt := []byte{0xda, 0x90, 0x45, 0xc3, 0x06, 0xb5, 0x99, 0x9f, 0xb6, 0xae, 0xfc, 0x14, 0xef, 0x27, 0x6e, 0x6a}
			encryptionKey = pbkdf2.Key([]byte(settings.Config.Auth.TotpSecret), salt, 4096, 32, sha256.New)
		}
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
