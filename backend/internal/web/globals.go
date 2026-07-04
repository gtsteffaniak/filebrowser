package web

import (
	"io/fs"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var (
	config      *settings.Settings
	assetFs     fs.FS
	accessStore *access.Storage
	shareStore  *share.Storage
	usersStore  *users.Storage
)

// InitGlobals sets package-level dependencies used by handlers and middleware.
func InitGlobals(cfg *settings.Settings, assets fs.FS, access *access.Storage, shares *share.Storage, users *users.Storage) {
	config = cfg
	assetFs = assets
	accessStore = access
	shareStore = shares
	usersStore = users
	activity.InitDeps(cfg, access, shares)
}
