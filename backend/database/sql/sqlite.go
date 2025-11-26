package sql

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// TempDB manages a temporary SQLite database for operations that need
// to stream large datasets without loading everything into memory.
// This can be used by any part of the codebase that needs temporary SQLite storage.
type TempDB struct {
	db   *sql.DB
	path string
	mu   sync.Mutex
}

// NewTempDB creates a new temporary SQLite database.
// If baseDir is empty, the database is created in the system temp directory.
// If baseDir is provided, the database is created in baseDir/sql/ subdirectory.
// The database will be cleaned up on Close().
//
// The database is optimized for bulk operations with:
// - WAL journal mode for better concurrency
// - NORMAL synchronous mode for better performance
// - Large cache size (64MB)
// - Temporary tables stored in memory
func NewTempDB(baseDir string) (*TempDB, error) {
	var dbDir string
	var tmpPath string

	if baseDir != "" {
		// Create sql subdirectory in the cache directory
		dbDir = filepath.Join(baseDir, "sql")
		if err := os.MkdirAll(dbDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create sql directory: %w", err)
		}
		// Create temporary file in the sql subdirectory
		tmpFile, err := os.CreateTemp(dbDir, "filebrowser-temp-*.db")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()
	} else {
		// Fallback to system temp directory
		tmpFile, err := os.CreateTemp("", "filebrowser-temp-*.db")
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()
	}

	// Open SQLite database with optimizations for bulk operations
	// Using PRAGMAs in connection string for better performance
	db, err := sql.Open("sqlite", tmpPath+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=cache_size(-64000)&_pragma=temp_store(MEMORY)")
	if err != nil {
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to open SQLite database: %w", err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
		os.Remove(tmpPath)
		return nil, fmt.Errorf("failed to ping SQLite database: %w", err)
	}

	return &TempDB{
		db:   db,
		path: tmpPath,
	}, nil
}

// DB returns the underlying *sql.DB connection.
// This allows callers to execute custom SQL queries if needed.
func (t *TempDB) DB() *sql.DB {
	return t.db
}

// BeginTransaction starts a transaction for bulk operations.
// The caller must call Commit() or Rollback() on the returned transaction.
// The mutex is NOT held during the transaction - caller is responsible for coordination.
func (t *TempDB) BeginTransaction() (*sql.Tx, error) {
	return t.db.Begin()
}

// Exec executes a SQL statement that doesn't return rows.
// This is a convenience method that handles locking.
func (t *TempDB) Exec(query string, args ...interface{}) (sql.Result, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.db.Exec(query, args...)
}

// Query executes a query that returns rows.
// This is a convenience method that handles locking.
func (t *TempDB) Query(query string, args ...interface{}) (*sql.Rows, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.db.Query(query, args...)
}

// QueryRow executes a query that is expected to return at most one row.
// This is a convenience method that handles locking.
func (t *TempDB) QueryRow(query string, args ...interface{}) *sql.Row {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.db.QueryRow(query, args...)
}

// Close closes the database connection and removes the temporary file.
// This should always be called when done with the database, typically in a defer statement.
func (t *TempDB) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.db != nil {
		if err := t.db.Close(); err != nil {
			os.Remove(t.path)
			return err
		}
	}

	return os.Remove(t.path)
}

// Path returns the path to the temporary database file.
// This is useful for debugging or if you need to inspect the database.
func (t *TempDB) Path() string {
	return t.path
}
