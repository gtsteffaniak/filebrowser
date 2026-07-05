// Package ports defines narrow interfaces for dependency injection.
// Domain packages depend on these interfaces; state.Store implements them.
package ports

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// UserReader loads persisted users (read-only).
type UserReader interface {
	GetUserByID(id uint64) (users.User, error)
	GetUserByUsername(username string) (users.User, error)
}

// UserWriter persists user changes. Handlers orchestrate writes; auth must not implement this.
type UserWriter interface {
	CreateUser(user *users.User, plaintextPassword string) error
	UpdateUser(user *users.User, plaintextPassword string, fields ...string) error
	DeleteUser(id uint64) error
}

// AccessGate enforces path-level access rules (no indexing types to avoid import cycles).
type AccessGate interface {
	AccessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool
	UpdateAccessRulesOnMove(sourcePath string, oldPath, newPath utils.IndexPath) (int, error)
}

// ShareReader loads share metadata for file and activity operations.
type ShareReader interface {
	GetShare(hash string) (share.Share, error)
	IsShared(source, path string, userID uint64) bool
}

// IndexMetaStore persists per-source index metadata (complexity, scanner state).
type IndexMetaStore interface {
	GetIndexInfo(path string) (dbindex.IndexInfo, error)
	SaveIndexInfo(info *dbindex.IndexInfo) error
	ResetAllIndexComplexities() error
}
