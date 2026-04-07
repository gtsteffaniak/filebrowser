package state

import (
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	// Separate mutexes for each resource type for better concurrency
	usersMux  sync.RWMutex
	sharesMux sync.RWMutex
	indexMux  sync.RWMutex

	// SQL store
	sqlStore *sqldb.SQLStore

	// In-memory caches
	usersByID   map[uint64]*users.User
	usersByName map[string]*users.User
	sharesByHash    map[string]*share.Link
	sharesByPath    map[string][]string // "source:path" -> []hash
	indexInfoByPath map[string]*dbindex.IndexInfo

	// Access storage (manages its own state)
	accessStorage *access.Storage
)

// Initialize loads all data from SQL into memory
func Initialize(dbPath string) (bool, error) {
	// Lock all mutexes during initialization
	usersMux.Lock()
	sharesMux.Lock()
	indexMux.Lock()
	defer usersMux.Unlock()
	defer sharesMux.Unlock()
	defer indexMux.Unlock()

	logger.Info("Initializing state management system...")
	var existingDb bool
	var err error
	sqlStore, existingDb, err = sqldb.NewSQLStore(dbPath)
	if err != nil {
		return false, fmt.Errorf("failed to initialize SQL store: %w", err)
	}

	// Initialize caches
	usersByID = make(map[uint64]*users.User)
	usersByName = make(map[string]*users.User)
	sharesByHash = make(map[string]*share.Link)
	sharesByPath = make(map[string][]string)
	indexInfoByPath = make(map[string]*dbindex.IndexInfo)

	logger.Debugf("Loading all data into memory...")

	// Load users
	usersList, err := sqlStore.ListUsers()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load users: %w", err)
	}
	for _, user := range usersList {
		usersByName[user.Username] = user
		if user.ID != 0 {
			usersByID[user.ID] = user
		}
	}
	logger.Debugf("Loaded %d users", len(usersList))

	users.SetUsernameToID(UserIDForUsername)

	// Load shares
	sharesList, err := sqlStore.ListAllShares()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load shares: %w", err)
	}
	for _, link := range sharesList {
		sharesByHash[link.Hash] = link
		pathKey := makePathKey(link.Source, link.Path)
		sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)
	}
	logger.Debugf("Loaded %d shares", len(sharesList))

	// Load index info
	allIndexInfo, err := sqlStore.ListAllIndexInfo()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load index info: %w", err)
	}
	for _, info := range allIndexInfo {
		indexInfoByPath[info.Path] = info
	}
	logger.Debugf("Loaded %d index info entries", len(allIndexInfo))

	// Initialize access storage
	accessStorage = &access.Storage{
		AllRules:      make(access.SourceRuleMap),
		Groups:        make(access.GroupMap),
		RevokedTokens: make(map[string]struct{}),
		HashedTokens:  make(map[string]uint64),
	}

	// Load access rules
	allRules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load access rules: %w", err)
	}
	accessStorage.AllRules = allRules
	logger.Debugf("Loaded access rules for %d sources", len(allRules))

	// Load groups
	allGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load groups: %w", err)
	}
	accessStorage.Groups = allGroups
	logger.Debugf("Loaded %d groups", len(allGroups))

	// Load tokens
	revokedTokens, err := sqlStore.GetAllRevokedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load revoked tokens: %w", err)
	}
	accessStorage.RevokedTokens = revokedTokens
	logger.Debugf("Loaded %d revoked tokens", len(revokedTokens))

	hashedTokens, err := sqlStore.GetAllHashedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load hashed tokens: %w", err)
	}
	accessStorage.HashedTokens = hashedTokens
	logger.Debugf("Loaded %d hashed tokens", len(hashedTokens))

	// Connect access storage to SQL
	accessStorage.SetSQLStore(sqlStore)

	// Initialize auth encryption key for TOTP
	err = auth.InitializeEncryption()
	if err != nil {
		return existingDb, fmt.Errorf("failed to initialize auth encryption: %w", err)
	}

	logger.Debugf("State management system initialized successfully")
	return existingDb, nil
}

// Close closes the underlying SQL store
func Close() error {
	// Lock all mutexes during close
	usersMux.Lock()
	sharesMux.Lock()
	indexMux.Lock()
	defer usersMux.Unlock()
	defer sharesMux.Unlock()
	defer indexMux.Unlock()

	users.SetUsernameToID(nil)
	if sqlStore != nil {
		return sqlStore.Close()
	}
	return nil
}

// GetAccessStorage returns the access storage for direct use
func GetAccessStorage() *access.Storage {
	return accessStorage
}

// GetShareStorage returns a share.Storage backed by state (for use with files.FileInfoFaster, etc.)
func GetShareStorage() *share.Storage {
	return share.NewStorage(shareBackend{}, nil)
}

// GetUsersStorage returns a users.Storage backed by state (for use with auth.GenerateOtpForUser, etc.)
func GetUsersStorage() *users.Storage {
	return users.NewStorage(usersBackend{})
}

// GetIndexingStorage returns a dbindex.Storage backed by state
func GetIndexingStorage() *dbindex.Storage {
	return dbindex.NewStorage(indexBackend{})
}

func makePathKey(source, path string) string {
	return source + ":" + path
}
