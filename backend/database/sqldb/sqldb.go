package sqldb

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/gtsteffaniak/go-logger/logger"
)

// SQLStore provides access to the SQLite database
type SQLStore struct {
	db *sql.DB
}

// NewSQLStore creates a new SQLStore and initializes the database
func NewSQLStore(dbPath string) (*SQLStore, bool, error) {
	// Check if database exists
	existed := dbExists(dbPath)

	// Ensure parent directory exists
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, existed, fmt.Errorf("failed to create database directory: %w", err)
	}

	// Open SQLite database
	db, err := sql.Open("sqlite3", fmt.Sprintf("file:%s?cache=shared&mode=rwc&_journal_mode=WAL", dbPath))
	if err != nil {
		return nil, existed, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		db.Close()
		return nil, existed, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Create schema if needed
	err = createSchema(db)
	if err != nil {
		db.Close()
		return nil, existed, err
	}

	// Initialize or check schema version
	err = initializeSchemaVersion(db)
	if err != nil {
		db.Close()
		return nil, existed, err
	}

	// Run migrations if needed
	version, err := getSchemaVersion(db)
	if err != nil {
		db.Close()
		return nil, existed, err
	}

	if version < currentSchemaVersion {
		logger.Infof("Running database migrations from version %d to %d", version, currentSchemaVersion)
		err = runMigrations(db, version)
		if err != nil {
			db.Close()
			return nil, existed, err
		}
	}

	store := &SQLStore{db: db}
	logger.Debugf("SQLite database initialized at %s", dbPath)

	return store, existed, nil
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
