package settings

import (
	"github.com/gtsteffaniak/filebrowser/errors"
	"github.com/gtsteffaniak/filebrowser/rules"
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
	if set.UserHomeBasePath == "" {
		set.UserHomeBasePath = DefaultUsersHomeBasePath
	}
	return set, nil
}

var defaultEvents = []string{
	"save",
	"copy",
	"rename",
	"upload",
	"delete",
}

// Save saves the settings for the current instance.
func (s *Storage) Save(set *Settings) error {
	if len(set.Key) == 0 {
		return errors.ErrEmptyKey
	}

	if set.UserDefaults.Locale == "" {
		set.UserDefaults.Locale = "en"
	}

	if set.UserDefaults.Commands == nil {
		set.UserDefaults.Commands = []string{}
	}

	if set.UserDefaults.ViewMode == "" {
		set.UserDefaults.ViewMode = "normal"
	}

	if set.Rules == nil {
		set.Rules = []rules.Rule{}
	}

	if set.Shell == nil {
		set.Shell = []string{}
	}

	if set.Commands == nil {
		set.Commands = map[string][]string{}
	}

	for _, event := range defaultEvents {
		if _, ok := set.Commands["before_"+event]; !ok {
			set.Commands["before_"+event] = []string{}
		}

		if _, ok := set.Commands["after_"+event]; !ok {
			set.Commands["after_"+event] = []string{}
		}
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
	ser.Clean()
	return s.back.SaveServer(ser)
}
