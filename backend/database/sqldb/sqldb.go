package sqldb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
	// SQLite driver is imported in driver_cgo.go or driver_nocgo.go based on build tags
)

// SQLStore provides access to the SQLite database
type SQLStore struct {
	db *sql.DB
}

// NewSQLStoreOpts configures NewSQLStoreWithOptions.
type NewSQLStoreOpts struct {
	// SkipQuickSetup skips creating the default admin user. Use when users will be
	// imported immediately (e.g. BoltDB → SQLite migration) to avoid UNIQUE username conflicts.
	SkipQuickSetup bool
}

// NewSQLStore creates a new SQLStore and initializes the database
func NewSQLStore(dbPath string) (*SQLStore, bool, error) {
	return NewSQLStoreWithOptions(dbPath, NewSQLStoreOpts{})
}

// NewSQLStoreWithOptions creates a new SQLStore with optional behavior (see NewSQLStoreOpts).
func NewSQLStoreWithOptions(dbPath string, opts NewSQLStoreOpts) (*SQLStore, bool, error) {
	// Check if database exists BEFORE opening it
	existingDb := dbExists(dbPath)

	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, existingDb, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open SQLite database with the appropriate driver
	db, err := sql.Open(SqliteDriver, fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbPath))
	if err != nil {
		return nil, existingDb, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		return nil, existingDb, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema if needed
	err = createSchema(db)
	if err != nil {
		db.Close()
		return nil, existingDb, err
	}

	// Initialize or check schema version
	err = initializeSchemaVersion(db)
	if err != nil {
		db.Close()
		return nil, existingDb, err
	}

	// Run migrations if needed
	version, err := getSchemaVersion(db)
	if err != nil {
		db.Close()
		return nil, existingDb, err
	}

	if version < currentSchemaVersion {
		logger.Infof("Running database migrations from version %d to %d", version, currentSchemaVersion)
		err = runMigrations(db, version)
		if err != nil {
			db.Close()
			return nil, existingDb, err
		}
	}

	store := &SQLStore{db: db}

	// Check if this is a new database by counting users
	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	isNewDB := (err != nil || userCount == 0)

	// Run quickSetup for new databases (no users exist)
	if isNewDB && !opts.SkipQuickSetup {
		err = store.quickSetup()
		if err != nil {
			db.Close()
			return nil, existingDb, fmt.Errorf("failed to run initial setup: %w", err)
		}
	}

	logger.Debugf("SQLite database initialized at %s", dbPath)

	return store, existingDb, nil
}

// Close closes the database connection
func (s *SQLStore) Close() error {
	return s.db.Close()
}

// DB returns the underlying *sql.DB for advanced operations
func (s *SQLStore) DB() *sql.DB {
	return s.db
}

// dbExists checks if a database file exists
func dbExists(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	return stat.Size() > 0
}

// currentTimestamp returns the current Unix timestamp
func currentTimestamp() int64 {
	return time.Now().Unix()
}

// BeginTx starts a new transaction
func (s *SQLStore) BeginTx() (*sql.Tx, error) {
	return s.db.Begin()
}

// quickSetup creates the initial admin user on first run
func (s *SQLStore) quickSetup() error {
	// Set IsFirstLoad flag based on environment
	if settings.Env.IsPlaywright || settings.Env.IsDevMode {
		settings.Env.IsFirstLoad = false
	} else {
		settings.Env.IsFirstLoad = true
	}

	// Generate auth key (always regenerate on new DB)
	settings.Config.Auth.Key = utils.GenerateKey()

	// Create admin user if password or noauth is enabled
	passwordAuth := settings.Config.Auth.Methods.PasswordAuth.Enabled
	noAuth := settings.Config.Auth.Methods.NoAuth

	if passwordAuth || noAuth {
		user := &users.User{}
		settings.ApplyUserDefaults(user)

		// Set admin username and password
		user.Username = settings.Config.Auth.AdminUsername
		if user.Username == "" {
			user.Username = "admin"
		}

		if settings.Config.Auth.AdminPassword == "" {
			settings.Config.Auth.AdminPassword = "admin"
		}

		user.Permissions.Admin = true
		user.LoginMethod = users.LoginMethodPassword

		// Set scopes for all sources (using Sources, not SourceMap)
		user.Scopes = []users.SourceScope{}
		for _, val := range settings.Config.Server.Sources {
			user.Scopes = append(user.Scopes, users.SourceScope{
				Name:  val.Path, // backend name is path
				Scope: "/",
			})
		}

		user.LockPassword = false
		user.Permissions = settings.AdminPerms()
		user.ShowFirstLogin = settings.Env.IsFirstLoad && user.Permissions.Admin

		logger.Debugf("Creating user as admin: %v", user.Username)

		// Hash the password before storing
		hashedPassword, hashErr := utils.HashPwd(settings.Config.Auth.AdminPassword)
		if hashErr != nil {
			return fmt.Errorf("failed to hash admin password: %w", hashErr)
		}
		user.Password = hashedPassword

		nid, err := users.NextRandomUserID()
		if err != nil {
			return fmt.Errorf("failed to allocate admin user id: %w", err)
		}
		user.ID = nid

		// Save the user directly to SQL
		err = s.CreateUser(user)
		if err != nil {
			return fmt.Errorf("failed to create admin user: %w", err)
		}
	}

	return nil
}
