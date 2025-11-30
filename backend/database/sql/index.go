package sql

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// IndexDB manages the SQLite database for the file index.
// It wraps the underlying sql.DB connection and provides type-safe methods.
// Uses DELETE journal mode for maximum write performance with SQLite's built-in locking for concurrency.
type IndexDB struct {
	*TempDB
}

// NewIndexDB creates a new index database in the cache directory.
// It uses the standard TempDB configuration optimized for performance.
func NewIndexDB(name string) (*IndexDB, error) {
	// Create a temp DB for indexing (ID based on source name)
	// Using "index_" prefix for clarity.
	// Start with 10MB cache - will be dynamically increased based on index complexity
	db, err := NewTempDB("index_"+name, &TempDBConfig{
		// cache_size: Negative values = pages, positive = KB
		// With 4KB page size: -2500 pages = 2500 * 4096 = ~10MB
		// Using 4KB pages for small entries reduces storage waste and RAM usage
		CacheSizeKB:   -2500,       // 10MB cache (2500 pages * 4KB = 10MB) - will be increased dynamically
		MmapSize:      100000000,   // 100MB mmap (memory-mapped I/O)
		Synchronous:   "OFF",       // OFF for maximum write performance - data integrity not critical for index
		TempStore:     "FILE",      // FILE instead of MEMORY
		JournalMode:   "DELETE",    // DELETE mode - faster writes, no WAL overhead, simpler for write-heavy workloads
		LockingMode:   "EXCLUSIVE", // NORMAL mode - allows concurrent reads, SQLite handles write locking automatically
		PageSize:      4096,        // 4KB page size - optimal for small entries (reduces storage waste)
		AutoVacuum:    "NONE",      // No vacuum overhead
		EnableLogging: true,
	})
	if err != nil {
		return nil, err
	}

	idxDB := &IndexDB{TempDB: db}

	// Set busy_timeout to 100ms for fast-fail behavior
	// We want ALL operations (read and write) to fail quickly so requests can fall back to filesystem reads
	// The index is a performance cache, not a critical dependency - filesystem is source of truth
	if _, err := db.Exec("PRAGMA busy_timeout = 100"); err != nil {
		idxDB.Close()
		return nil, fmt.Errorf("failed to set busy_timeout: %w", err)
	}

	if err := idxDB.CreateIndexTable(); err != nil {
		idxDB.Close()
		return nil, err
	}

	return idxDB, nil
}

// CreateIndexTable creates the main table for storing file metadata.
func (db *IndexDB) CreateIndexTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS index_items (
		path TEXT PRIMARY KEY,
		parent_path TEXT NOT NULL,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		mod_time INTEGER NOT NULL,
		type TEXT NOT NULL,
		is_dir BOOLEAN NOT NULL,
		is_hidden BOOLEAN NOT NULL,
		has_preview BOOLEAN NOT NULL
	);
	
	CREATE INDEX IF NOT EXISTS idx_parent_path ON index_items(parent_path);
	CREATE INDEX IF NOT EXISTS idx_size ON index_items(size);
	CREATE INDEX IF NOT EXISTS idx_name ON index_items(name);
	`
	_, err := db.Exec(query)
	return err
}

// InsertItem adds or updates an item in the index.
// SQLite handles locking automatically with NORMAL locking mode.
func (db *IndexDB) InsertItem(path string, info *iteminfo.FileInfo) error {
	query := `
	INSERT OR REPLACE INTO index_items (path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	parentPath := getParentPath(path)
	_, err := db.Exec(query,
		path,
		parentPath,
		info.Name,
		info.Size,
		info.ModTime.Unix(),
		info.Type,
		info.Type == "directory",
		info.Hidden,
		info.HasPreview,
	)
	return err
}

// BulkInsertItems inserts multiple items in a single transaction.
// Database errors (busy/locked) are treated as soft failures - the filesystem is the source of truth.
// Returns nil on success or soft failure (busy/locked), error only on hard failures.
func (db *IndexDB) BulkInsertItems(items []*iteminfo.FileInfo) error {
	// Quick attempt with no retries - if DB is busy, just skip the update
	// The next request will try again, and filesystem reads are always available as fallback

	startTime := time.Now()
	logger.Debugf("[DB_TX] BulkInsertItems: Starting transaction for %d items", len(items))

	tx, err := db.BeginTransaction()
	if err != nil {
		// Soft failure: DB is busy or locked, skip this update
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_TX] BulkInsertItems: BeginTransaction failed (DB busy/locked), skipping - took %v", time.Since(startTime))
			return nil // Non-fatal: filesystem will be used as fallback
		}
		return err // Hard failure: something is wrong with the DB
	}

	stmt, err := tx.Prepare(`
	INSERT OR REPLACE INTO index_items (path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		tx.Rollback()
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	for _, info := range items {
		parentPath := getParentPath(info.Path)
		_, err := stmt.Exec(
			info.Path,
			parentPath,
			info.Name,
			info.Size,
			info.ModTime.Unix(),
			info.Type,
			info.Type == "directory",
			info.Hidden,
			info.HasPreview,
		)
		if err != nil {
			stmt.Close()
			tx.Rollback()
			if isBusyError(err) || isTransactionError(err) {
				return nil // Non-fatal
			}
			return err
		}
	}

	stmt.Close()

	// Try to commit
	if err := tx.Commit(); err != nil {
		tx.Rollback() // Clean up failed transaction
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	logger.Debugf("[DB_TX] BulkInsertItems: SUCCESS - %d items in %v", len(items), time.Since(startTime))
	return nil
}

// isBusyError checks if an error is SQLITE_BUSY (error code 5)
func isBusyError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLITE_BUSY is error code 5, and modernc.org/sqlite returns it as "database is locked (5)"
	return strings.Contains(errStr, "database is locked") || strings.Contains(errStr, "SQLITE_BUSY") || strings.Contains(errStr, "(5)")
}

// isTransactionError checks if an error is related to nested transactions (error code 1)
func isTransactionError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// SQLITE_ERROR for transaction issues is error code 1
	return strings.Contains(errStr, "cannot start a transaction within a transaction") ||
		strings.Contains(errStr, "cannot commit") ||
		strings.Contains(errStr, "(1)")
}

// GetItem retrieves a single item by path.
// Returns nil on database busy/lock errors (non-fatal).
func (db *IndexDB) GetItem(path string) (*iteminfo.FileInfo, error) {
	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE path = ?
	`
	row := db.QueryRow(query, path)
	item, err := scanItem(row)
	if err != nil {
		// Soft failure: DB is busy or locked, return nil
		// Caller will handle missing data by fetching from filesystem
		if isBusyError(err) || isTransactionError(err) {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

// GetItemsByPaths retrieves multiple items by their paths in a single query.
// This is more efficient than calling GetItem multiple times.
// Returns empty map on database busy/lock errors (non-fatal).
func (db *IndexDB) GetItemsByPaths(paths []string) (map[string]*iteminfo.FileInfo, error) {
	if len(paths) == 0 {
		return make(map[string]*iteminfo.FileInfo), nil
	}

	// Build query with IN clause
	placeholders := make([]string, len(paths))
	args := make([]interface{}, len(paths))
	for i, path := range paths {
		placeholders[i] = "?"
		args[i] = path
	}

	query := fmt.Sprintf(`
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE path IN (%s)
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty map
		// Caller will handle missing data (e.g., skip size updates, fetch from filesystem)
		if isBusyError(err) || isTransactionError(err) {
			return make(map[string]*iteminfo.FileInfo), nil
		}
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]*iteminfo.FileInfo)
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		result[item.Path] = item
	}

	return result, nil
}

// BulkUpdateSizes updates the sizes of multiple items in a single transaction.
// This is optimized for updating parent directory sizes after file operations.
// Includes retry logic for SQLITE_BUSY errors to handle concurrent write operations.
func (db *IndexDB) BulkUpdateSizes(pathSizeUpdates map[string]int64) error {
	if len(pathSizeUpdates) == 0 {
		return nil
	}

	// Quick attempt with no retries - if DB is busy, just skip the update
	// Parent sizes are less critical than file existence, filesystem is source of truth

	startTime := time.Now()
	logger.Debugf("[DB_TX] BulkUpdateSizes: Starting transaction for %d paths", len(pathSizeUpdates))

	tx, err := db.BeginTransaction()
	if err != nil {
		// Soft failure: DB is busy or locked, skip this update
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_TX] BulkUpdateSizes: BeginTransaction failed (DB busy/locked), skipping - took %v", time.Since(startTime))
			return nil // Non-fatal: sizes can be recalculated later
		}
		return err
	}

	stmt, err := tx.Prepare(`
	UPDATE index_items 
	SET size = size + ?
	WHERE path = ?
	`)
	if err != nil {
		tx.Rollback()
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	for path, sizeDelta := range pathSizeUpdates {
		if sizeDelta == 0 {
			continue
		}
		_, err := stmt.Exec(sizeDelta, path)
		if err != nil {
			stmt.Close()
			tx.Rollback()
			if isBusyError(err) || isTransactionError(err) {
				return nil // Non-fatal
			}
			return err
		}
	}

	stmt.Close()

	// Try to commit
	if err := tx.Commit(); err != nil {
		tx.Rollback() // Clean up failed transaction
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	return nil
}

// GetDirectoryFiles retrieves all children of a directory.
// Returns empty slice on database busy/lock errors (non-fatal).
func (db *IndexDB) GetDirectoryChildren(dirPath string) ([]*iteminfo.FileInfo, error) {
	// Ensure dirPath has trailing slash for parent_path comparison if needed,
	// but our helper getParentPath handles stripping.
	// We assume stored paths for files match the convention.

	// If the dirPath comes in as "/foo/bar", we expect children to have parent_path "/foo/bar/"
	// Wait, parent_path logic needs to be consistent.
	// If file is "/foo/bar/baz.txt", parent is "/foo/bar/".

	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE parent_path = ?
	ORDER BY is_dir DESC, name ASC
	`

	rows, err := db.Query(query, dirPath)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty slice
		// Caller will handle missing data by fetching from filesystem
		if isBusyError(err) || isTransactionError(err) {
			return []*iteminfo.FileInfo{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var children []*iteminfo.FileInfo
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		children = append(children, item)
	}
	return children, nil
}

// DeleteItem removes an item and optionally its children (recursive).
// SQLite handles locking automatically with NORMAL locking mode.
func (db *IndexDB) DeleteItem(path string, recursive bool) error {
	if !recursive {
		_, err := db.Exec("DELETE FROM index_items WHERE path = ?", path)
		return err
	}

	// Recursive delete
	// Since we store full paths, we can just delete where path starts with dirPath
	// Ensure path has trailing slash for directory prefix matching
	dirPrefix := path
	if !strings.HasSuffix(dirPrefix, "/") {
		dirPrefix += "/"
	}

	// Delete the item itself
	if _, err := db.Exec("DELETE FROM index_items WHERE path = ?", path); err != nil {
		return err
	}

	// Delete children
	_, err := db.Exec("DELETE FROM index_items WHERE path GLOB ?", dirPrefix+"*")
	return err
}

// UpdateCacheSize updates the SQLite cache size at runtime.
// cacheSizeMB is the desired cache size in megabytes.
// With 4KB page size, cacheSizeMB * 1024 / 4 = pages needed
// Note: PRAGMA statements don't support parameterized queries, so we use fmt.Sprintf
func (db *IndexDB) UpdateCacheSize(cacheSizeMB int) error {
	if cacheSizeMB < 1 {
		return fmt.Errorf("cache size must be at least 1MB, got %dMB", cacheSizeMB)
	}

	// Calculate pages: MB * 1024 KB/MB / 4 KB/page = MB * 256 pages
	// Use negative value to specify pages (SQLite convention)
	cachePages := -(cacheSizeMB * 256)

	// PRAGMA doesn't support parameterized queries, must use string formatting
	// Format directly in the query string - no placeholders allowed for PRAGMA
	query := fmt.Sprintf("PRAGMA cache_size = %d", cachePages)
	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to set cache_size to %dMB (%d pages): %w", cacheSizeMB, cachePages, err)
	}
	return nil
}

// GetFilesBySize retrieves all files with a specific size, optionally filtered by path prefix.
// Used for duplicate detection - returns files ordered by name for efficient grouping.
// Returns empty slice on database busy/lock errors (non-fatal).
func (db *IndexDB) GetFilesBySize(size int64, pathPrefix string) ([]*iteminfo.FileInfo, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE size = ? AND is_dir = 0 AND path GLOB ?
		ORDER BY name
		`
		args = []interface{}{size, pathPrefix + "*"}
	} else {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE size = ? AND is_dir = 0
		ORDER BY name
		`
		args = []interface{}{size}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty slice
		if isBusyError(err) || isTransactionError(err) {
			return []*iteminfo.FileInfo{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var files []*iteminfo.FileInfo
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		files = append(files, item)
	}

	return files, rows.Err()
}

// GetSizeGroupsForDuplicates queries for all size groups that have 2+ files.
// Returns sizes in descending order (largest first) and a count map.
// Optionally filters by path prefix for scoped searches.
// Returns empty results on database busy/lock errors (non-fatal).
func (db *IndexDB) GetSizeGroupsForDuplicates(minSize int64, pathPrefix string) ([]int64, map[int64]int, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT size, COUNT(*) as count
		FROM index_items
		WHERE size >= ? AND is_dir = 0 AND path GLOB ?
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
		`
		args = []interface{}{minSize, pathPrefix + "*"}
	} else {
		query = `
		SELECT size, COUNT(*) as count
		FROM index_items
		WHERE size >= ? AND is_dir = 0
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
		`
		args = []interface{}{minSize}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty results
		if isBusyError(err) || isTransactionError(err) {
			return []int64{}, make(map[int64]int), nil
		}
		return nil, nil, err
	}
	defer rows.Close()

	var sizes []int64
	sizeCounts := make(map[int64]int)
	for rows.Next() {
		var size int64
		var count int
		if err := rows.Scan(&size, &count); err != nil {
			return nil, nil, err
		}
		sizes = append(sizes, size)
		sizeCounts[size] = count
	}

	return sizes, sizeCounts, rows.Err()
}

// Helper functions

func getParentPath(path string) string {
	if path == "/" {
		return ""
	}
	// If it's a directory with trailing slash, remove it to get parent
	path = strings.TrimSuffix(path, "/")
	dir := path[:strings.LastIndex(path, "/")+1]
	return dir
}

func scanItem(scanner interface{ Scan(...interface{}) error }) (*iteminfo.FileInfo, error) {
	var info iteminfo.FileInfo
	var modTime int64
	var isDir bool

	err := scanner.Scan(
		&info.Path,
		&info.Name,
		&info.Size,
		&modTime,
		&info.Type,
		&isDir,
		&info.Hidden,
		&info.HasPreview,
	)
	if err != nil {
		return nil, err
	}
	info.ModTime = time.Unix(modTime, 0)
	return &info, nil
}

func scanRow(rows *sql.Rows) (*iteminfo.FileInfo, error) {
	return scanItem(rows)
}
