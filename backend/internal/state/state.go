package state

import (
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	// Separate mutexes for each resource type for better concurrency
	usersMux  sync.RWMutex
	sharesMux sync.RWMutex
	indexMux  sync.RWMutex

	// sqlDb is the SQLite persistence layer. Only state package code should call it directly;
	// everyone else goes through exported state.* helpers and the in-memory caches below.
	sqlDb *sqldb.SQLStore

	// In-memory caches (authoritative at runtime after Initialize)
	usersByID       map[uint64]*users.User
	usersByName     map[string]*users.User
	sharesByHash    map[string]*share.Share
	sharesByPath    map[string][]string // "source:path" -> []hash
	indexInfoByPath map[string]*dbindex.IndexInfo

	// accessDb holds access rules, groups, and token hashes in memory with write-through to sqlDb.
	accessDb *access.Storage
)

// Initialize loads all data from SQL into memory and sets the default store handle.
// Prefer state.Open when you need the *Store for dependency injection.
func Initialize(dbPath string) (bool, error) {
	_, existingDb, err := Open(dbPath)
	return existingDb, err
}

func initialize(dbPath string) (bool, error) {
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
	sqlDb, existingDb, err = sqldb.NewSQLStore(dbPath)
	if err != nil {
		return false, fmt.Errorf("failed to initialize SQL database: %w", err)
	}

	// Initialize caches
	usersByID = make(map[uint64]*users.User)
	usersByName = make(map[string]*users.User)
	sharesByHash = make(map[string]*share.Share)
	sharesByPath = make(map[string][]string)
	indexInfoByPath = make(map[string]*dbindex.IndexInfo)

	logger.Debugf("Loading all data into memory...")

	// Load users
	usersList, err := sqlDb.ListUsers()
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
	sharesList, err := sqlDb.ListAllShares()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load shares: %w", err)
	}
	for _, link := range sharesList {
		sharesByHash[link.Hash] = link
		pathKey := makePathKey(link.SourcePath, link.Path)
		sharesByPath[pathKey] = append(sharesByPath[pathKey], link.Hash)
	}
	logger.Debugf("Loaded %d shares", len(sharesList))

	// Load index info
	allIndexInfo, err := sqlDb.ListAllIndexInfo()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load index info: %w", err)
	}
	for _, info := range allIndexInfo {
		indexInfoByPath[info.Path] = info
	}
	logger.Debugf("Loaded %d index info entries", len(allIndexInfo))

	// Initialize access rules cache
	accessDb = &access.Storage{
		AllRules:      make(access.SourceRuleMap),
		Groups:        make(access.GroupMap),
		RevokedTokens: make(map[string]struct{}),
		HashedTokens:  make(map[string]uint64),
	}

	allRules, err := sqlDb.GetAllAccessRules()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load access rules: %w", err)
	}
	accessDb.AllRules = allRules
	logger.Debugf("Loaded access rules for %d sources", len(allRules))

	allGroups, err := sqlDb.GetAllGroups()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load groups: %w", err)
	}
	accessDb.Groups = allGroups
	logger.Debugf("Loaded %d groups", len(allGroups))

	revokedTokens, err := sqlDb.GetAllRevokedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load revoked tokens: %w", err)
	}
	accessDb.RevokedTokens = revokedTokens
	logger.Debugf("Loaded %d revoked tokens", len(revokedTokens))

	hashedTokens, err := sqlDb.GetAllHashedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load hashed tokens: %w", err)
	}
	accessDb.HashedTokens = hashedTokens
	logger.Debugf("Loaded %d hashed tokens", len(hashedTokens))

	accessDb.SetSQLStore(sqlDb)

	err = auth.InitializeEncryption()
	if err != nil {
		return existingDb, fmt.Errorf("failed to initialize auth encryption: %w", err)
	}

	logger.Debugf("State management system initialized successfully")

	if err := InitAnalyticsSettings(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize analytics settings: %w", err)
	}

	InitActivityRecorder(settings.Config.Server.DatabaseV2)

	return existingDb, nil
}

// Close closes the underlying SQL database
func Close() error {
	usersMux.Lock()
	sharesMux.Lock()
	indexMux.Lock()
	defer usersMux.Unlock()
	defer sharesMux.Unlock()
	defer indexMux.Unlock()

	users.SetUsernameToID(nil)
	StopActivityRecorder()
	if sqlDb != nil {
		return sqlDb.Close()
	}
	return nil
}

func makePathKey(source, path string) string {
	return source + ":" + path
}
