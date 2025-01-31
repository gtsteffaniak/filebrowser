package bolt

import (
	"github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/share"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) (*auth.Storage, *users.Storage, *share.Storage, *settings.Storage, error) {
	userStore := users.NewStorage(usersBackend{db: db})
	shareStore := share.NewStorage(shareBackend{db: db})
	settingsStore := settings.NewStorage(settingsBackend{db: db})
	authStore, err := auth.NewStorage(authBackend{db: db}, userStore)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return authStore, userStore, shareStore, settingsStore, nil
}
