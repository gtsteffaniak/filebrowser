package bolt

import (
	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// Storage is a storage powered by a Backend which makes the necessary
// verifications when fetching and saving data to ensure consistency.
type BoltStore struct {
	Users    *users.Storage
	Share    *share.Storage
	Auth     *auth.Storage
	Settings *settings.Storage
	Access   *access.Storage
	Indexing *dbindex.Storage
}

// NewStorage creates a storage.Storage based on Bolt DB.
func NewStorage(db *storm.DB) (*BoltStore, error) {
	userStore := users.NewStorage(usersBackend{db: db})
	authStore, err := auth.NewStorage(authBackend{db: db}, userStore)
	if err != nil {
		return nil, err
	}
	return &BoltStore{
		Users:    userStore,
		Share:    share.NewStorage(shareBackend{db: db}, userStore),
		Auth:     authStore,
		Settings: settings.NewStorage(settingsBackend{db: db}),
		Access:   access.NewStorage(db, userStore),
		Indexing: dbindex.NewStorage(indexingBackend{db: db}),
	}, nil
}
