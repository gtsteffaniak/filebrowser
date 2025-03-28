package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
)

// StorageBackend is a settings storage backend.
type StorageBackend interface {
	Get() (*Settings, error)
	Save(*Settings) error
	GetServer() (*Server, error)
	SaveServer(*Server) error
}

// Storage is a settings storage.
type Storage struct {
	back StorageBackend
}

// NewStorage creates a settings storage from a backend.
func NewStorage(back StorageBackend) *Storage {
	return &Storage{back: back}
}

// Get returns the settings for the current instance.
func (s *Storage) Get() (*Settings, error) {
	set, err := s.back.Get()
	if err != nil {
		return nil, err
	}
	if set.Server.UserHomeBasePath == "" {
		set.Server.UserHomeBasePath = DefaultUsersHomeBasePath
	}
	return set, nil
}

// Save saves the settings for the current instance.
func (s *Storage) Save(set *Settings) error {
	if len(set.Auth.Key) == 0 {
		return errors.ErrEmptyKey
	}

	if set.UserDefaults.Locale == "" {
		set.UserDefaults.Locale = "en"
	}

	if set.UserDefaults.ViewMode == "" {
		set.UserDefaults.ViewMode = "normal"
	}

	err := s.back.Save(set)
	if err != nil {
		return err
	}

	return nil
}

// GetServer wraps StorageBackend.GetServer.
func (s *Storage) GetServer() (*Server, error) {
	return s.back.GetServer()
}

// SaveServer wraps StorageBackend.SaveServer and adds some verification.
func (s *Storage) SaveServer(ser *Server) error {
	return s.back.SaveServer(ser)
}
