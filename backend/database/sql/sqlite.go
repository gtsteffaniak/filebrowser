package sql

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// TempDB manages a temporary SQLite database for operations that need
// to stream large datasets without loading everything into memory.
// This can be used by any part of the codebase that needs temporary SQLite storage.
type TempDB struct {
	db        *sql.DB
	path      string
	mu        sync.Mutex
	startTime time.Time
	config    *TempDBConfig
}

// TempDBConfig holds configuration options for temporary SQLite databases.
type TempDBConfig struct {
	// CacheSizeKB is the page cache size in KB. Negative values are in pages.
	// For one-time databases, a smaller cache (e.g., -2000 = ~8MB) is often sufficient.
	// Default: -2000 (approximately 8MB)
	CacheSizeKB int

	// MmapSize is the memory-mapped I/O size in bytes. Set to 0 to disable mmap.
	// For databases that fit in RAM, set this larger than the expected DB size.
	// Default: 2GB (2147483648 bytes)
	MmapSize int64

	// Synchronous controls the synchronous mode. OFF is fastest but less safe.
	// For temporary databases, OFF is acceptable.
	// Default: OFF
	Synchronous string

	// TempStore controls where temporary tables and indices are stored.
	// Valid values: "FILE" (default), "MEMORY", "DEFAULT"
	// Default: FILE (temporary tables stored on disk)
	TempStore string

	// JournalMode controls the journal mode. WAL is better for concurrent reads/writes.
	// DELETE is faster for write-heavy single-writer workloads.
	// Valid values: "DELETE", "WAL", "TRUNCATE", "PERSIST", "MEMORY", "OFF"
	// Default: WAL
	JournalMode string

	// LockingMode controls the locking mode. EXCLUSIVE prevents other processes from accessing.
	// Valid values: "NORMAL", "EXCLUSIVE"
	// Default: NORMAL
	LockingMode string

	// PageSize sets the database page size in bytes. Larger pages improve write performance.
	// Valid values: 512, 1024, 2048, 4096, 8192, 16384, 32768, 65536
	// Default: 4096 (SQLite default)
	PageSize int

	// AutoVacuum controls automatic vacuuming. NONE disables it for better write performance.
	// Valid values: "NONE", "FULL", "INCREMENTAL"
	// Default: NONE
	AutoVacuum string

	// EnableLogging enables performance logging for debugging.
	// Default: false
	EnableLogging bool
}

// mergeConfig merges the provided config with defaults, returning a new config.
// If provided config is nil or empty, returns default config.
func mergeConfig(provided *TempDBConfig) *TempDBConfig {
	defaults := &TempDBConfig{
		CacheSizeKB:   -2000, // ~8MB, appropriate for one-time databases
		MmapSize:      0,     // Default to 0 (disabled) to prevent high memory usage
		Synchronous:   "OFF",
		TempStore:     "FILE", // Default to FILE, not MEMORY
		JournalMode:   "WAL",  // WAL for better concurrency by default
		LockingMode:   "NORMAL",
		PageSize:      4096, // SQLite default
		AutoVacuum:    "NONE",
		EnableLogging: false,
	}

	if provided == nil {
		return defaults
	}

	merged := *defaults // Copy defaults

	// Override with provided values if they are non-zero/non-empty
	if provided.CacheSizeKB != 0 {
		merged.CacheSizeKB = provided.CacheSizeKB
	}
	if provided.MmapSize != 0 {
		merged.MmapSize = provided.MmapSize
	}
	if provided.Synchronous != "" {
		merged.Synchronous = provided.Synchronous
	}
	if provided.TempStore != "" {
		merged.TempStore = provided.TempStore
	}
	if provided.JournalMode != "" {
		merged.JournalMode = provided.JournalMode
	}
	if provided.LockingMode != "" {
		merged.LockingMode = provided.LockingMode
	}
	if provided.PageSize != 0 {
		merged.PageSize = provided.PageSize
	}
	if provided.AutoVacuum != "" {
		merged.AutoVacuum = provided.AutoVacuum
	}
	merged.EnableLogging = provided.EnableLogging

	return &merged
}

// NewTempDB creates a new temporary SQLite database.
// The database is created in the cache directory's sql/ subdirectory.
// The database will be cleaned up on Close().
//
// The database is optimized for bulk write-then-read operations with:
// - WAL journal mode for better concurrency
// - Configurable cache size (default: ~8MB for one-time DBs)
// - Memory-mapped I/O for faster access
// - OFF synchronous mode for maximum write performance
// - Configurable temp_store (default: FILE, can be set to MEMORY via config)
func NewTempDB(id string, config ...*TempDBConfig) (*TempDB, error) {
	startTime := time.Now()

	// Merge provided config with defaults
	var providedConfig *TempDBConfig
	if len(config) > 0 {
		providedConfig = config[0]
	}
	cfg := mergeConfig(providedConfig)

	// Create sql subdirectory in the cache directory
	dbDir := filepath.Join(settings.Config.Server.CacheDir, "sql")
	if err := os.MkdirAll(dbDir, fileutils.PermDir); err != nil {
		return nil, fmt.Errorf("failed to create sql directory: %w", err)
	}

	// Create temporary file in the sql subdirectory
	tmpFile, err := os.CreateTemp(dbDir, fmt.Sprintf("%s.db", id))
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	// Open SQLite database with basic connection string
	// We'll set PRAGMAs after connection for better control and logging
	db, err := sql.Open(sqliteDriverName, tmpPath)
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

	// Limit connection pool to 1 for SQLite - it's a file-based database and multiple connections
	// can cause locking issues. With busy_timeout, SQLite will queue operations automatically.
	if cfg.LockingMode == "EXCLUSIVE" {
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
	}
	// Apply optimizations via PRAGMA statements
	// Execute them individually for compatibility and better error reporting
	// IMPORTANT: page_size must be set BEFORE any tables are created
	pragmaStart := time.Now()

	pragmas := []struct {
		sql string
		err string
	}{
		{fmt.Sprintf("PRAGMA journal_mode = %s;", cfg.JournalMode), "failed to set journal_mode"},
		{fmt.Sprintf("PRAGMA cache_size = %d;", cfg.CacheSizeKB), "failed to set cache_size"},
		{fmt.Sprintf("PRAGMA synchronous = %s;", cfg.Synchronous), "failed to set synchronous"},
		{fmt.Sprintf("PRAGMA temp_store = %s;", cfg.TempStore), "failed to set temp_store"},
		{fmt.Sprintf("PRAGMA locking_mode = %s;", cfg.LockingMode), "failed to set locking_mode"},
		{fmt.Sprintf("PRAGMA auto_vacuum = %s;", cfg.AutoVacuum), "failed to set auto_vacuum"},
	}

	// Page size must be set before any tables are created
	if cfg.PageSize > 0 {
		pragmas = append([]struct {
			sql string
			err string
		}{{fmt.Sprintf("PRAGMA page_size = %d;", cfg.PageSize), "failed to set page_size"}}, pragmas...)
	}

	// Always set mmap_size, even if 0 (which explicitly disables it)
	pragmas = append(pragmas, struct {
		sql string
		err string
	}{fmt.Sprintf("PRAGMA mmap_size = %d;", cfg.MmapSize), "failed to set mmap_size"})

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma.sql); err != nil {
			db.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("%s: %w", pragma.err, err)
		}
	}

	pragmaDuration := time.Since(pragmaStart)

	// Log configuration if enabled
	if cfg.EnableLogging {
		logger.Debugf("[TempDB:%s] Created with cache_size=%d KB, mmap_size=%d bytes, synchronous=%s, temp_store=%s, journal_mode=%s, locking_mode=%s, page_size=%d, auto_vacuum=%s (setup took %v)",
			id, cfg.CacheSizeKB, cfg.MmapSize, cfg.Synchronous, cfg.TempStore, cfg.JournalMode, cfg.LockingMode, cfg.PageSize, cfg.AutoVacuum, pragmaDuration)
	}

	return &TempDB{
		db:        db,
		path:      tmpPath,
		startTime: startTime,
		config:    cfg,
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
// If logging is enabled, it will log the total lifetime and file size for performance analysis.
func (t *TempDB) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.config != nil && t.config.EnableLogging {
		totalDuration := time.Since(t.startTime)
		fileInfo, err := os.Stat(t.path)
		var fileSize int64
		if err == nil {
			fileSize = fileInfo.Size()
		}
		logger.Debugf("[TempDB] Closed after %v, final size: %d bytes (%.2f MB)",
			totalDuration, fileSize, float64(fileSize)/(1024*1024))
	}

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
