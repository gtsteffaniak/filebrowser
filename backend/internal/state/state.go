package state

import (
	"fmt"
	"sync"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/dbindex"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/sqldb"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

var (
	// Serializes Open/Close so tests and startup cannot interleave lifecycle or sqlDb swaps.
	stateLifecycleMu sync.Mutex

	// Separate mutexes for each resource type for better concurrency
	usersMux  sync.RWMutex
	sharesMux sync.RWMutex
	indexMux  sync.RWMutex

	// sqlDb is the SQLite persistence layer. Only state package code should call it directly;
	// everyone else goes through exported state.* helpers and the caches below.
	sqlDb *sqldb.SQLStore

	// In-memory caches (authoritative at runtime after Initialize for shares/access/index; users use TTL cache)
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
	if !settings.Env.IsCLIMode {
		logger.Info("Initializing state management system...")
	}

	store, existingDb, err := sqldb.NewSQLStore(dbPath)
	if err != nil {
		return false, fmt.Errorf("failed to initialize SQL database: %w", err)
	}
	sqlDb = store

	if err = InitUserDefaultsSettings(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize user defaults settings: %w", err)
	}

	if err = InitSourceAccessDefaults(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize source access defaults: %w", err)
	}

	if err = InitSidebarLinkDefaults(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize sidebar link defaults: %w", err)
	}

	if err = InitToolAccessDefaults(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize tool access defaults: %w", err)
	}

	if err = InitShareDefaultsSettings(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize share defaults settings: %w", err)
	}

	var userCount int
	if countErr := sqlDb.DB().QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount); countErr != nil {
		return existingDb, fmt.Errorf("failed to count users: %w", countErr)
	}
	if userCount == 0 {
		if err = QuickSetup(); err != nil {
			return existingDb, fmt.Errorf("failed to run initial setup: %w", err)
		}
	}

	logger.Debugf("Loading shares and index data into memory...")

	users.SetUsernameToID(UserIDForUsername)

	newSharesByHash := make(map[string]*share.Share)
	newSharesByPath := make(map[string][]string)
	sharesList, err := sqlDb.ListAllShares()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load shares: %w", err)
	}
	for _, link := range sharesList {
		newSharesByHash[link.Hash] = link
		pathKey := makePathKey(link.SourcePath, link.Path)
		newSharesByPath[pathKey] = append(newSharesByPath[pathKey], link.Hash)
	}
	logger.Debugf("Loaded %d shares", len(sharesList))

	newIndexInfoByPath := make(map[string]*dbindex.IndexInfo)
	allIndexInfo, err := sqlDb.ListAllIndexInfo()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load index info: %w", err)
	}
	for _, info := range allIndexInfo {
		newIndexInfoByPath[info.Path] = info
	}
	logger.Debugf("Loaded %d index info entries", len(allIndexInfo))

	newAccessDb := &access.Storage{
		AllRules:      make(access.SourceRuleMap),
		Groups:        make(access.GroupMap),
		RevokedTokens: make(map[string]struct{}),
		HashedTokens:  make(map[string]uint64),
	}

	allRules, err := sqlDb.GetAllAccessRules()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load access rules: %w", err)
	}
	newAccessDb.AllRules = allRules
	logger.Debugf("Loaded access rules for %d sources", len(allRules))

	allGroups, err := sqlDb.GetAllGroups()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load groups: %w", err)
	}
	newAccessDb.Groups = allGroups
	logger.Debugf("Loaded %d groups", len(allGroups))

	revokedTokens, err := sqlDb.GetAllRevokedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load revoked tokens: %w", err)
	}
	newAccessDb.RevokedTokens = revokedTokens
	logger.Debugf("Loaded %d revoked tokens", len(revokedTokens))

	hashedTokens, err := sqlDb.GetAllHashedTokens()
	if err != nil {
		return existingDb, fmt.Errorf("failed to load hashed tokens: %w", err)
	}
	newAccessDb.HashedTokens = hashedTokens
	logger.Debugf("Loaded %d hashed tokens", len(hashedTokens))

	newAccessDb.SetSQLStore(sqlDb)

	usersMux.Lock()
	sharesMux.Lock()
	indexMux.Lock()
	sharesByHash = newSharesByHash
	sharesByPath = newSharesByPath
	indexInfoByPath = newIndexInfoByPath
	accessDb = newAccessDb
	usersMux.Unlock()
	sharesMux.Unlock()
	indexMux.Unlock()

	err = auth.InitializeEncryption()
	if err != nil {
		return existingDb, fmt.Errorf("failed to initialize auth encryption: %w", err)
	}

	logger.Debugf("State management system initialized successfully")

	if err := InitAnalyticsSettings(); err != nil {
		return existingDb, fmt.Errorf("failed to initialize analytics settings: %w", err)
	}

	InitActivityRecorder(settings.Config.Server.DatabaseV2)

	if err := InitQuotas(settings.Config.Server.DatabaseV2); err != nil {
		return existingDb, fmt.Errorf("failed to initialize quotas: %w", err)
	}

	return existingDb, nil
}

// Close closes the underlying SQL database
func Close() error {
	stateLifecycleMu.Lock()
	defer stateLifecycleMu.Unlock()

	usersMux.Lock()
	sharesMux.Lock()
	indexMux.Lock()
	defer usersMux.Unlock()
	defer sharesMux.Unlock()
	defer indexMux.Unlock()

	users.SetUsernameToID(nil)
	clearUserRecordCache()
	StopActivityRecorder()
	StopQuotaFlusher()

	var err error
	if sqlDb != nil {
		err = sqlDb.Close()
		sqlDb = nil
	}
	defaultStore = nil
	return err
}

func makePathKey(source, path string) string {
	return source + ":" + path
}
