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
	mux sync.RWMutex
	
	// SQL store
	sqlStore *sqldb.SQLStore
	
	// In-memory caches
	usersByID       map[uint]*users.User
	usersByName     map[string]*users.User
	sharesByHash    map[string]*share.Link
	sharesByPath    map[string][]string // "source:path" -> []hash
	indexInfoByPath map[string]*dbindex.IndexInfo
	
	// Access storage (manages its own state)
	accessStorage *access.Storage
	
	// Settings cache
	settingsCache    interface{}
	serverCache      interface{}
	authMethodsCache map[string][]byte
)

// Initialize loads all data from SQL into memory
func Initialize(dbPath string) error {
	mux.Lock()
	defer mux.Unlock()
	
	logger.Info("Initializing state management system...")
	
	// Open SQL store
	var hasDB bool
	var err error
	sqlStore, hasDB, err = sqldb.NewSQLStore(dbPath)
	if err != nil {
		return fmt.Errorf("failed to initialize SQL store: %w", err)
	}
	
	// Initialize caches
	usersByID = make(map[uint]*users.User)
	usersByName = make(map[string]*users.User)
	sharesByHash = make(map[string]*share.Link)
	sharesByPath = make(map[string][]string)
	indexInfoByPath = make(map[string]*dbindex.IndexInfo)
	authMethodsCache = make(map[string][]byte)
	
	logger.Info("Loading all data into memory...")
	
	// Load users
	usersList, err := sqlStore.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}
	for _, user := range usersList {
		usersByID[user.ID] = user
		usersByName[user.Username] = user
	}
	logger.Infof("Loaded %d users", len(usersList))
	
	// Load shares
	sharesList, err := sqlStore.ListAllShares()
	if err != nil {
		return fmt.Errorf("failed to load shares: %w", err)
	}
	for _, link := range sharesList {
		sharesByHash[link.Hash] = link
		pathKey := makePathKey(link.Source, link.Path)
		sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)
	}
	logger.Infof("Loaded %d shares", len(sharesList))
	
	// Load index info
	allIndexInfo, err := sqlStore.ListAllIndexInfo()
	if err != nil {
		return fmt.Errorf("failed to load index info: %w", err)
	}
	for _, info := range allIndexInfo {
		indexInfoByPath[info.Path] = info
	}
	logger.Infof("Loaded %d index info entries", len(allIndexInfo))
	
	// Initialize access storage
	accessStorage = &access.Storage{
		AllRules:      make(access.SourceRuleMap),
		Groups:        make(access.GroupMap),
		RevokedTokens: make(map[string]struct{}),
		HashedTokens:  make(map[string]uint),
	}
	
	// Load access rules
	allRules, err := sqlStore.GetAllAccessRules()
	if err != nil {
		return fmt.Errorf("failed to load access rules: %w", err)
	}
	accessStorage.AllRules = allRules
	logger.Infof("Loaded access rules for %d sources", len(allRules))
	
	// Load groups
	allGroups, err := sqlStore.GetAllGroups()
	if err != nil {
		return fmt.Errorf("failed to load groups: %w", err)
	}
	accessStorage.Groups = allGroups
	logger.Infof("Loaded %d groups", len(allGroups))
	
	// Load tokens
	revokedTokens, err := sqlStore.GetAllRevokedTokens()
	if err != nil {
		return fmt.Errorf("failed to load revoked tokens: %w", err)
	}
	accessStorage.RevokedTokens = revokedTokens
	logger.Infof("Loaded %d revoked tokens", len(revokedTokens))
	
	hashedTokens, err := sqlStore.GetAllHashedTokens()
	if err != nil {
		return fmt.Errorf("failed to load hashed tokens: %w", err)
	}
	accessStorage.HashedTokens = hashedTokens
	logger.Infof("Loaded %d hashed tokens", len(hashedTokens))
	
	// Connect access storage to SQL
	accessStorage.SetSQLStore(sqlStore)
	
	logger.Info("State management system initialized successfully")
	
	if !hasDB {
		logger.Info("New database created")
	}
	
	return nil
}

// Close closes the underlying SQL store
func Close() error {
	mux.Lock()
	defer mux.Unlock()
	
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

// GetAuthStorage returns an auth.Storage for password/proxy/noauth methods
func GetAuthStorage() (*auth.Storage, error) {
	return auth.NewStorage(authBackend{}, GetUsersStorage())
}

// GetIndexingStorage returns a dbindex.Storage backed by state
func GetIndexingStorage() *dbindex.Storage {
	return dbindex.NewStorage(indexBackend{})
}

func makePathKey(source, path string) string {
	return source + ":" + path
}
