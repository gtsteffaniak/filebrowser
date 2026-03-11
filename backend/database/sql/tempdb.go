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
	// SQLite driver is imported in driver_cgo.go or driver_nocgo.go based on build tags
)

const (
	// soft heap limit in bytes
	defaultSoftHeapLimitBytes = 32 * 1024 * 1024 // 32MB
	// Pager cache target expressed as number of 4KB pages (negative = pages).
	defaultCacheSizePages = -4096 // ~16MB
)

func init() {
	if SqliteDriver == "sqlite3" {
		logger.Debugf("CGO SQLite driver initialized")
	} else {
		logger.Debugf("Default SQLite driver initialized")
	}
}

// TempDB manages a temporary SQLite database for operations that need
// to stream large datasets without loading everything into memory.
// This can be used by any part of the codebase that needs temporary SQLite storage.
type TempDB struct {
	db        *sql.DB
	path      string
	mu        sync.Mutex
	startTime time.Time
	config    *TempDBConfig
	BatchSize int // Exported batch size for bulk operations (from config)
}

// TempDBConfig holds configuration options for temporary SQLite databases.
type TempDBConfig struct {
	// BatchSize is the number of items to batch for bulk insert.
	// Default: 2500
	BatchSize int

	// CacheSizeKB is the page cache size in KB. Negative values are in pages.
	// For one-time databases, a smaller cache (e.g., -4096 = ~16MB) is often sufficient.
	// Default: -4096 (approximately 16MB)
	CacheSizeKB int

	// MmapSize is the memory-mapped I/O size in bytes. Set to 0 to disable mmap.
	// For databases that fit in RAM, set this larger than the expected DB size.
	// Default: 0 (disabled)
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

	// SoftHeapLimitBytes, when >0, sets PRAGMA soft_heap_limit so that SQLite
	// proactively frees caches before exceeding this many bytes. Zero disables it.
	SoftHeapLimitBytes int64

	// HardHeapLimitBytes, when >0, sets PRAGMA hard_heap_limit as a hard cap on SQLite's heap.
	// Operations will fail if they would exceed this limit. Zero disables it.
	HardHeapLimitBytes int64

	// CacheSpillThreshold sets PRAGMA cache_spill threshold in pages. When cache exceeds both
	// this threshold and cache_size, SQLite will spill dirty pages to disk.
	// Set to 0 to use default behavior (spill enabled but no specific threshold).
	// Default: 500 pages (~2MB) to trigger early spilling and reduce memory usage.
	CacheSpillThreshold int

	// WalAutocheckpoint sets PRAGMA wal_autocheckpoint in pages. Controls how many pages
	// can accumulate in the WAL file before an automatic checkpoint.
	// Default: 1000 pages (~4MB). Larger values reduce checkpoint frequency.
	WalAutocheckpoint int

	// JournalSizeLimit sets PRAGMA journal_size_limit in bytes. Limits the maximum size
	// of the WAL journal file. Set to -1 for no limit, 0 for default.
	// Default: 0 (uses SQLite default)
	JournalSizeLimit int64

	// If empty, a temp file with random suffix will be created.
	// The file will not be deleted on Close() if this is set.
	PersistentFile string
}

// mergeConfig merges the provided config with defaults, returning a new config.
// If provided config is nil or empty, returns default config.
func mergeConfig(provided *TempDBConfig) *TempDBConfig {
	defaults := &TempDBConfig{
		BatchSize:   2500,
		CacheSizeKB: defaultCacheSizePages, // ~16MB pager cache
		MmapSize:    0,                     // Disable mmap to keep RSS predictable
		Synchronous: "OFF",
		TempStore:   "FILE", // Default to FILE, not MEMORY
		JournalMode: "WAL",  // WAL for better concurrency by default
		LockingMode: "NORMAL",
		PageSize:    4096, // SQLite default
		AutoVacuum:  "NONE",
		// Soft heap limit keeps SQLite from retaining excessive pager cache memory.
		SoftHeapLimitBytes: defaultSoftHeapLimitBytes,
		// Hard heap limit provides a hard cap - operations fail if exceeded.
		HardHeapLimitBytes: 0, // Disabled by default, can be set per-database
		// Cache spill threshold triggers early spilling to reduce memory usage.
		CacheSpillThreshold: 500, // ~2MB threshold to trigger early spilling
	}

	if provided == nil {
		return defaults
	}

	merged := *defaults // Copy defaults

	// Override with provided values if they are non-zero/non-empty
	if provided.BatchSize != 0 {
		merged.BatchSize = provided.BatchSize
	}
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
	if provided.HardHeapLimitBytes != 0 {
		merged.HardHeapLimitBytes = provided.HardHeapLimitBytes
	}
	if provided.CacheSpillThreshold != 0 {
		merged.CacheSpillThreshold = provided.CacheSpillThreshold
	}
	// SoftHeapLimitBytes: use provided value if non-zero (default is 32MB, but provided might be different)
	if provided.SoftHeapLimitBytes != 0 {
		merged.SoftHeapLimitBytes = provided.SoftHeapLimitBytes
	}
	// PersistentFile: always copy if non-empty (this determines if file is persistent or temp)
	if provided.PersistentFile != "" {
		merged.PersistentFile = provided.PersistentFile
	}

	return &merged
}

// NewTempDB creates a new temporary SQLite database.
// The database is created in the cache directory's sql/ subdirectory.
// The database will be cleaned up on Close().
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

	// Determine database file path
	var tmpPath string
	if cfg.PersistentFile != "" {
		// Use fixed filename for persistent databases
		tmpPath = filepath.Join(dbDir, cfg.PersistentFile)
		// File might already exist (persistent), that's fine
	} else {
		// Create temporary file with random suffix
		tmpFile, err := os.CreateTemp(dbDir, fmt.Sprintf("%s.db", id))
		if err != nil {
			return nil, fmt.Errorf("failed to create temp file: %w", err)
		}
		tmpPath = tmpFile.Name()
		tmpFile.Close()
	}

	// Open SQLite database with basic connection string
	// Driver is selected at compile time: "sqlite3" (CGO) or "sqlite" (pure Go)
	db, err := sql.Open(SqliteDriver, tmpPath)
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

	// Calculate cache_size in pages (negative value) to ensure consistent unit handling
	// SQLite's cache_size: positive = KB, negative = pages
	// When using positive KB values, SQLite seems to have issues with cache_spill being less than cache_size
	// So we convert to pages (negative) to ensure both cache_size and cache_spill use the same units
	cacheSizeInPages := cfg.CacheSizeKB
	if cfg.CacheSizeKB > 0 {
		// Convert KB to pages: KB / 4 = pages, then negate to indicate pages
		cacheSizeInPages = -(cfg.CacheSizeKB / 4)
	}

	basePragmas := []struct {
		sql string
		err string
	}{
		{fmt.Sprintf("PRAGMA journal_mode = %s;", cfg.JournalMode), "failed to set journal_mode"},
		{fmt.Sprintf("PRAGMA cache_size = %d;", cacheSizeInPages), "failed to set cache_size"},
		{fmt.Sprintf("PRAGMA synchronous = %s;", cfg.Synchronous), "failed to set synchronous"},
		{fmt.Sprintf("PRAGMA temp_store = %s;", cfg.TempStore), "failed to set temp_store"},
		{fmt.Sprintf("PRAGMA locking_mode = %s;", cfg.LockingMode), "failed to set locking_mode"},
		{fmt.Sprintf("PRAGMA auto_vacuum = %s;", cfg.AutoVacuum), "failed to set auto_vacuum"},
	}

	// Memory management PRAGMAs - set early to constrain memory usage
	if cfg.SoftHeapLimitBytes > 0 {
		basePragmas = append([]struct {
			sql string
			err string
		}{{fmt.Sprintf("PRAGMA soft_heap_limit = %d;", cfg.SoftHeapLimitBytes), "failed to set soft_heap_limit"}}, basePragmas...)
	}

	if cfg.HardHeapLimitBytes > 0 {
		basePragmas = append([]struct {
			sql string
			err string
		}{{fmt.Sprintf("PRAGMA hard_heap_limit = %d;", cfg.HardHeapLimitBytes), "failed to set hard_heap_limit"}}, basePragmas...)
	}

	// Cache spill will be set later, after all other pragmas, to avoid interaction issues

	// Page size must be set before any tables are created
	if cfg.PageSize > 0 {
		basePragmas = append([]struct {
			sql string
			err string
		}{{fmt.Sprintf("PRAGMA page_size = %d;", cfg.PageSize), "failed to set page_size"}}, basePragmas...)
	}

	// Always explicitly set mmap_size to ensure it's disabled (0) or enabled as configured
	// Setting it to 0 explicitly disables memory-mapped I/O, which helps reduce OS page cache usage
	basePragmas = append(basePragmas, struct {
		sql string
		err string
	}{fmt.Sprintf("PRAGMA mmap_size = %d;", cfg.MmapSize), "failed to set mmap_size"})

	// WAL autocheckpoint - controls checkpoint frequency in WAL mode
	if cfg.WalAutocheckpoint > 0 {
		basePragmas = append(basePragmas, struct {
			sql string
			err string
		}{fmt.Sprintf("PRAGMA wal_autocheckpoint = %d;", cfg.WalAutocheckpoint), "failed to set wal_autocheckpoint"})
	}

	// Journal size limit - limits WAL file growth
	if cfg.JournalSizeLimit > 0 {
		basePragmas = append(basePragmas, struct {
			sql string
			err string
		}{fmt.Sprintf("PRAGMA journal_size_limit = %d;", cfg.JournalSizeLimit), "failed to set journal_size_limit"})
	} else if cfg.JournalSizeLimit == -1 {
		// -1 means no limit
		basePragmas = append(basePragmas, struct {
			sql string
			err string
		}{"PRAGMA journal_size_limit = -1;", "failed to set journal_size_limit"})
	}

	for _, pragma := range basePragmas {
		if _, err := db.Exec(pragma.sql); err != nil {
			db.Close()
			os.Remove(tmpPath)
			return nil, fmt.Errorf("%s: %w", pragma.err, err)
		}
	}

	// Set cache_spill LAST, after all other pragmas are configured
	// SQLite may adjust cache_spill based on internal constraints or formulas
	if cfg.CacheSpillThreshold > 0 {
		// Try setting cache_spill multiple times with different approaches
		// Approach 1: Standard syntax
		if _, err := db.Exec(fmt.Sprintf("PRAGMA cache_spill = %d;", cfg.CacheSpillThreshold)); err != nil {
			logger.Warningf("[SQLITE_CONFIG] Failed to set cache_spill with standard syntax: %v", err)
		}

		// Immediately verify what value was actually set
		var actualValue int64
		if err := db.QueryRow("PRAGMA cache_spill").Scan(&actualValue); err == nil {
			if actualValue != int64(cfg.CacheSpillThreshold) {
				// SQLite adjusted our value - calculate what percentage it used
				percentage := float64(actualValue) / float64(-cacheSizeInPages) * 100.0
				logger.Warningf("[SQLITE_CONFIG] SQLite adjusted cache_spill from %d to %d pages (%.1f%% of cache_size). This may be due to internal constraints.",
					cfg.CacheSpillThreshold, actualValue, percentage)
			}
		}
	} else {
		// Enable cache_spill with default behavior
		if _, err := db.Exec("PRAGMA cache_spill = 1;"); err != nil {
			logger.Warningf("[SQLITE_CONFIG] Failed to enable cache_spill: %v", err)
		}
	}

	// Verify PRAGMA settings were applied correctly
	if err := verifyPragmaSettings(db, cfg); err != nil {
		logger.Warningf("Failed to verify PRAGMA settings: %v", err)
	}

	return &TempDB{
		db:        db,
		path:      tmpPath,
		startTime: startTime,
		config:    cfg,
		BatchSize: cfg.BatchSize, // Export batch size for external packages
	}, nil
}

// DB returns the underlying *sql.DB connection.
// This allows callers to execute custom SQL queries if needed.
func (t *TempDB) DB() *sql.DB {
	return t.db
}

// BeginTransaction starts a transaction for bulk operations.
// The caller must call Commit() or Rollback() on the returned transaction.
// IMPORTANT: The mutex is acquired here and MUST be released by calling
// t.mu.Unlock() after commit/rollback to ensure exclusive transaction access.
func (t *TempDB) BeginTransaction() (*sql.Tx, error) {
	t.mu.Lock()
	// NOTE: Mutex is NOT unlocked here! Caller must unlock after commit/rollback
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

// Close closes the database connection and removes the temporary file if it is not persistent.
func (t *TempDB) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.db != nil {
		if err := t.db.Close(); err != nil {
			// Only remove file if it's not persistent
			if t.config != nil && t.config.PersistentFile == "" {
				os.Remove(t.path)
			}
			return err
		}
	}

	// Only remove file if it's not persistent
	if t.config != nil && t.config.PersistentFile == "" {
		return os.Remove(t.path)
	}
	return nil
}

// verifyPragmaSettings reads back PRAGMA values to verify they were applied correctly
func verifyPragmaSettings(db *sql.DB, cfg *TempDBConfig) error {
	var cacheSize, mmapSize, softHeapLimit, hardHeapLimit, cacheSpill int64
	var journalMode, synchronous, tempStore, lockingMode, autoVacuum string

	// Read back PRAGMA values
	if err := db.QueryRow("PRAGMA cache_size").Scan(&cacheSize); err != nil {
		return fmt.Errorf("failed to read cache_size: %w", err)
	}
	if err := db.QueryRow("PRAGMA mmap_size").Scan(&mmapSize); err != nil {
		return fmt.Errorf("failed to read mmap_size: %w", err)
	}
	if err := db.QueryRow("PRAGMA soft_heap_limit").Scan(&softHeapLimit); err != nil {
		return fmt.Errorf("failed to read soft_heap_limit: %w", err)
	}
	if err := db.QueryRow("PRAGMA hard_heap_limit").Scan(&hardHeapLimit); err != nil {
		return fmt.Errorf("failed to read hard_heap_limit: %w", err)
	}
	if err := db.QueryRow("PRAGMA cache_spill").Scan(&cacheSpill); err != nil {
		return fmt.Errorf("failed to read cache_spill: %w", err)
	}
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&journalMode); err != nil {
		return fmt.Errorf("failed to read journal_mode: %w", err)
	}
	if err := db.QueryRow("PRAGMA synchronous").Scan(&synchronous); err != nil {
		return fmt.Errorf("failed to read synchronous: %w", err)
	}
	if err := db.QueryRow("PRAGMA temp_store").Scan(&tempStore); err != nil {
		return fmt.Errorf("failed to read temp_store: %w", err)
	}
	if err := db.QueryRow("PRAGMA locking_mode").Scan(&lockingMode); err != nil {
		return fmt.Errorf("failed to read locking_mode: %w", err)
	}
	if err := db.QueryRow("PRAGMA auto_vacuum").Scan(&autoVacuum); err != nil {
		return fmt.Errorf("failed to read auto_vacuum: %w", err)
	}

	// Convert cache_size from pages to KB (negative = pages, positive = KB)
	cacheSizeKB := cacheSize
	if cacheSize < 0 {
		cacheSizeKB = -cacheSize * 4 // pages * 4KB per page
	}

	// Expected cache_spill value in pages (cache_spill is always specified and returned in pages)
	expectedCacheSpill := int64(cfg.CacheSpillThreshold)

	// Convert numeric PRAGMA values to human-readable strings
	tempStoreMap := map[string]string{"0": "DEFAULT", "1": "FILE", "2": "MEMORY"}
	synchronousMap := map[string]string{"0": "OFF", "1": "NORMAL", "2": "FULL", "3": "EXTRA"}
	autoVacuumMap := map[string]string{"0": "NONE", "1": "FULL", "2": "INCREMENTAL"}

	tempStoreStr := tempStoreMap[tempStore]
	if tempStoreStr == "" {
		tempStoreStr = tempStore
	}
	synchronousStr := synchronousMap[synchronous]
	if synchronousStr == "" {
		synchronousStr = synchronous
	}
	autoVacuumStr := autoVacuumMap[autoVacuum]
	if autoVacuumStr == "" {
		autoVacuumStr = autoVacuum
	}

	// Log all settings
	logger.Debugf("[SQLITE_CONFIG] Database PRAGMA settings verified:")
	logger.Debugf("  cache_size      : %d KB", cacheSizeKB)
	logger.Debugf("  mmap_size       : %d bytes", mmapSize)
	logger.Debugf("  soft_heap_limit : %d bytes", softHeapLimit)
	logger.Debugf("  hard_heap_limit : %d bytes", hardHeapLimit)
	logger.Debugf("  cache_spill     : %d pages", cacheSpill)
	logger.Debugf("  journal_mode    : %s", journalMode)
	logger.Debugf("  synchronous     : %s", synchronousStr)
	logger.Debugf("  temp_store      : %s", tempStoreStr)
	logger.Debugf("  locking_mode    : %s", lockingMode)
	logger.Debugf("  auto_vacuum     : %s", autoVacuumStr)

	// Verify critical settings match
	if cfg.CacheSizeKB != 0 {
		expectedCache := int64(cfg.CacheSizeKB)
		if expectedCache < 0 {
			expectedCache = -expectedCache * 4 // Convert pages to KB
		}
		if cacheSizeKB != expectedCache {
			logger.Warningf("[SQLITE_CONFIG] cache_size mismatch: got %d KB, expected %d KB", cacheSizeKB, expectedCache)
		}
	}
	if cfg.MmapSize > 0 && mmapSize != cfg.MmapSize {
		logger.Warningf("[SQLITE_CONFIG] mmap_size mismatch: got %d bytes, expected %d bytes", mmapSize, cfg.MmapSize)
	}
	if cfg.SoftHeapLimitBytes > 0 && softHeapLimit != cfg.SoftHeapLimitBytes {
		logger.Warningf("[SQLITE_CONFIG] soft_heap_limit mismatch: got %d bytes, expected %d bytes", softHeapLimit, cfg.SoftHeapLimitBytes)
	}
	if cfg.HardHeapLimitBytes > 0 && hardHeapLimit != cfg.HardHeapLimitBytes {
		logger.Warningf("[SQLITE_CONFIG] hard_heap_limit mismatch: got %d bytes, expected %d bytes", hardHeapLimit, cfg.HardHeapLimitBytes)
	}
	// Verify cache_spill matches expected value (accounting for unit conversion)
	if cfg.CacheSpillThreshold > 0 && cacheSpill != expectedCacheSpill {
		logger.Warningf("[SQLITE_CONFIG] cache_spill mismatch: got %d pages, expected %d pages (from threshold %d pages)", cacheSpill, expectedCacheSpill, cfg.CacheSpillThreshold)
		// If cache_spill equals cache_size in pages, it means SQLite ignored our setting
		var cacheSizePages int64
		if cfg.CacheSizeKB > 0 {
			cacheSizePages = int64(cfg.CacheSizeKB) / 4 // KB to pages
		} else {
			cacheSizePages = -int64(cfg.CacheSizeKB) // Already in pages (negative)
		}
		if cacheSpill == cacheSizePages {
			logger.Warningf("[SQLITE_CONFIG] cache_spill equals cache_size (%d pages), indicating SQLite may have ignored our setting", cacheSizePages)
		}
	}

	// Verify string-based PRAGMA settings match (using converted values)
	if synchronousStr != cfg.Synchronous {
		logger.Warningf("[SQLITE_CONFIG] synchronous mismatch: got %s, expected %s", synchronousStr, cfg.Synchronous)
	}
	if tempStoreStr != cfg.TempStore {
		logger.Warningf("[SQLITE_CONFIG] temp_store mismatch: got %s, expected %s", tempStoreStr, cfg.TempStore)
	}
	if autoVacuumStr != cfg.AutoVacuum {
		logger.Warningf("[SQLITE_CONFIG] auto_vacuum mismatch: got %s, expected %s", autoVacuumStr, cfg.AutoVacuum)
	}

	return nil
}

// Path returns the path to the temporary database file.
// This is useful for debugging or if you need to inspect the database.
func (t *TempDB) Path() string {
	return t.path
}
