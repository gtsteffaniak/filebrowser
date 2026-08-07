package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// Store is the runtime handle for the state gateway. It implements ports interfaces
// by delegating to the package-level cache and sqlDb helpers.
type Store struct{}

var defaultStore *Store

// Default returns the store opened by Open/Initialize, or nil before startup.
func Default() *Store {
	return defaultStore
}

// Open loads all persisted data and returns the store handle for dependency injection.
func Open(dbPath string) (*Store, bool, error) {
	existingDb, err := initialize(dbPath)
	if err != nil {
		return nil, existingDb, err
	}
	defaultStore = &Store{}
	return defaultStore, existingDb, nil
}

// --- ports.UserReader ---

func (s *Store) GetUserByID(id uint64) (users.User, error) {
	return GetUserByID(id)
}

func (s *Store) GetUserByUsername(username string) (users.User, error) {
	return GetUserByUsername(username)
}

// --- ports.UserWriter ---

func (s *Store) CreateUser(user *users.User, plaintextPassword string) error {
	return CreateUser(user, plaintextPassword)
}

func (s *Store) UpdateUser(user *users.User, plaintextPassword string, fields ...string) error {
	return UpdateUser(user, plaintextPassword, fields...)
}

func (s *Store) DeleteUser(id uint64) error {
	return DeleteUser(id)
}

// --- ports.AccessGate ---

func (s *Store) AccessPermitted(sourcePath string, indexPath utils.IndexPath, username string) bool {
	return AccessPermitted(sourcePath, indexPath, username)
}

func (s *Store) UpdateAccessRulesOnMove(sourcePath string, oldPath, newPath utils.IndexPath) (int, error) {
	return UpdateAccessRulesOnMove(sourcePath, oldPath, newPath)
}

// CheckChildItemAccess implements files.ChildAccessChecker.
func (s *Store) CheckChildItemAccess(response *iteminfo.FileInfo, idx *indexing.Index, username string) error {
	return CheckChildItemAccess(response, idx, username)
}

// --- ports.ShareReader ---

func (s *Store) GetShare(hash string) (share.Share, error) {
	return GetShare(hash)
}

func (s *Store) IsShared(source, path string, userID uint64) bool {
	return IsShared(source, path, userID)
}

// --- ports.IndexMetaStore ---

func (s *Store) GetIndexInfo(path string) (dbindex.IndexInfo, error) {
	return GetIndexInfo(path)
}

func (s *Store) SaveIndexInfo(info *dbindex.IndexInfo) error {
	return SaveIndexInfo(info)
}

func (s *Store) ResetAllIndexComplexities() error {
	return ResetAllIndexComplexities()
}
