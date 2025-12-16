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
	db, err := NewTempDB("index_"+name, &TempDBConfig{
		// cache_size: Negative values = pages, positive = KB
		// With 4KB page size: -625 pages = 625 * 4096 = ~2.5MB
		// Using 4KB pages for small entries reduces storage waste and RAM usage
		CacheSizeKB:   -625,        // 2.5MB cache (625 pages * 4KB = 2.5MB) - will be increased dynamically up to 25MB
		MmapSize:      0,           // Disable mmap - use page cache only for controlled memory usage
		Synchronous:   "OFF",       // OFF for maximum write performance - data integrity not critical for index
		TempStore:     "FILE",      // FILE instead of MEMORY
		JournalMode:   "DELETE",    // DELETE mode - faster writes, no WAL overhead, simpler for write-heavy workloads
		LockingMode:   "EXCLUSIVE", // EXCLUSIVE mode - single writer, no contention
		PageSize:      4096,        // 4KB page size - optimal for small entries (reduces storage waste)
		AutoVacuum:    "NONE",      // No vacuum overhead (periodic manual VACUUM recommended)
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

func (db *IndexDB) CreateIndexTable() error {
	query := `
	CREATE TABLE IF NOT EXISTS index_items (
		source TEXT NOT NULL,
		path TEXT NOT NULL,
		parent_path TEXT NOT NULL,
		name TEXT NOT NULL,
		size INTEGER NOT NULL,
		mod_time INTEGER NOT NULL,
		type TEXT NOT NULL,
		is_dir BOOLEAN NOT NULL,
		is_hidden BOOLEAN NOT NULL,
		has_preview BOOLEAN NOT NULL,
		last_updated INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (source, path)
	);
	
	CREATE INDEX IF NOT EXISTS idx_source ON index_items(source);
	CREATE INDEX IF NOT EXISTS idx_source_parent_path ON index_items(source, parent_path);
	CREATE INDEX IF NOT EXISTS idx_source_size ON index_items(source, size);
	CREATE INDEX IF NOT EXISTS idx_source_size_type ON index_items(source, size, type);
	CREATE INDEX IF NOT EXISTS idx_name ON index_items(name);
	CREATE INDEX IF NOT EXISTS idx_last_updated ON index_items(source, last_updated);
	`
	_, err := db.Exec(query)
	return err
}

func (db *IndexDB) InsertItem(source, path string, info *iteminfo.FileInfo) error {
	query := `
	INSERT OR REPLACE INTO index_items (source, path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	parentPath := getParentPath(path)
	_, err := db.Exec(query,
		source,
		path,
		parentPath,
		info.Name,
		info.Size,
		info.ModTime.Unix(),
		info.Type,
		info.Type == "directory",
		info.Hidden,
		info.HasPreview,
		time.Now().Unix(),
	)
	if err != nil {
		if !isBusyError(err) && !isTransactionError(err) {
			logger.Errorf("InsertItem failed for source=%s path=%s: %v", source, path, err)
		}
	}
	return err
}

// BulkInsertItems inserts multiple items in a single transaction for a specific source.
// Database errors (busy/locked) are treated as soft failures - the filesystem is the source of truth.
// Returns nil on success or soft failure (busy/locked), error only on hard failures.
func (db *IndexDB) BulkInsertItems(source string, items []*iteminfo.FileInfo) error {
	// Quick attempt with no retries - if DB is busy, just skip the update
	// The next request will try again, and filesystem reads are always available as fallback

	startTime := time.Now()

	// Log directory inserts for debugging move/copy operations
	dirPaths := make([]string, 0)
	for _, info := range items {
		if info.Type == "directory" {
			dirPaths = append(dirPaths, info.Path)
		}
	}
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
	INSERT OR REPLACE INTO index_items (source, path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return nil
		}
		return err
	}

	nowUnix := time.Now().Unix()
	for _, info := range items {
		parentPath := getParentPath(info.Path)
		_, err := stmt.Exec(
			source,
			info.Path,
			parentPath,
			info.Name,
			info.Size,
			info.ModTime.Unix(),
			info.Type,
			info.Type == "directory",
			info.Hidden,
			info.HasPreview,
			nowUnix,
		)
		if err != nil {
			stmt.Close()
			if isBusyError(err) || isTransactionError(err) {
				return nil // Non-fatal
			}
			return err
		}
	}

	stmt.Close()

	// Try to commit
	if err := tx.Commit(); err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

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

// GetItem retrieves a single item by path for a specific source.
// Returns nil on database busy/lock errors (non-fatal).
func (db *IndexDB) GetItem(source, path string) (*iteminfo.FileInfo, error) {
	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE source = ? AND path = ?
	`
	row := db.QueryRow(query, source, path)
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

// GetItemsByPaths retrieves multiple items by their paths in a single query for a specific source.
// This is more efficient than calling GetItem multiple times.
// Returns empty map on database busy/lock errors (non-fatal).
func (db *IndexDB) GetItemsByPaths(source string, paths []string) (map[string]*iteminfo.FileInfo, error) {
	if len(paths) == 0 {
		return make(map[string]*iteminfo.FileInfo), nil
	}

	// Build query with IN clause
	placeholders := make([]string, len(paths))
	args := make([]interface{}, len(paths)+1)
	args[0] = source
	for i, path := range paths {
		placeholders[i] = "?"
		args[i+1] = path
	}

	query := fmt.Sprintf(`
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE source = ? AND path IN (%s)
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

// BulkUpdateSizes updates the sizes of multiple items in a single transaction for a specific source.
// This is optimized for updating parent directory sizes after file operations.
// Includes retry logic for SQLITE_BUSY errors to handle concurrent write operations.
func (db *IndexDB) BulkUpdateSizes(source string, pathSizeUpdates map[string]int64) error {
	if len(pathSizeUpdates) == 0 {
		return nil
	}

	// Quick attempt with no retries - if DB is busy, just skip the update
	// Parent sizes are less critical than file existence, filesystem is source of truth

	startTime := time.Now()
	logger.Debugf("[DB_TX] BulkUpdateSizes: Starting transaction for %d paths (source: %s)", len(pathSizeUpdates), source)

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
	SET size = size + ?, last_updated = ?
	WHERE source = ? AND path = ?
	`)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	nowUnix := time.Now().Unix()
	for path, sizeDelta := range pathSizeUpdates {
		if sizeDelta == 0 {
			continue
		}
		_, err := stmt.Exec(sizeDelta, nowUnix, source, path)
		if err != nil {
			stmt.Close()
			if isBusyError(err) || isTransactionError(err) {
				return nil // Non-fatal
			}
			return err
		}
	}

	stmt.Close()

	// Try to commit
	if err := tx.Commit(); err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return nil // Non-fatal
		}
		return err
	}

	return nil
}

// GetDirectoryChildren retrieves all children of a directory for a specific source.
// Returns empty slice on database busy/lock errors (non-fatal).
func (db *IndexDB) GetDirectoryChildren(source, dirPath string) ([]*iteminfo.FileInfo, error) {
	// Ensure dirPath has trailing slash for parent_path comparison if needed,
	// but our helper getParentPath handles stripping.
	// We assume stored paths for files match the convention.

	// If the dirPath comes in as "/foo/bar", we expect children to have parent_path "/foo/bar/"
	// Wait, parent_path logic needs to be consistent.
	// If file is "/foo/bar/baz.txt", parent is "/foo/bar/".

	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE source = ? AND parent_path = ?
	ORDER BY is_dir DESC, name ASC
	`

	rows, err := db.Query(query, source, dirPath)
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

// DeleteItem removes an item and optionally its children (recursive) for a specific source.
// SQLite handles locking automatically with NORMAL locking mode.
func (db *IndexDB) DeleteItem(source, path string, recursive bool) error {
	if !recursive {
		_, err := db.Exec("DELETE FROM index_items WHERE source = ? AND path = ?", source, path)
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
	if _, err := db.Exec("DELETE FROM index_items WHERE source = ? AND path = ?", source, path); err != nil {
		return err
	}

	// Delete children
	_, err := db.Exec("DELETE FROM index_items WHERE source = ? AND path GLOB ?", source, dirPrefix+"*")
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

// Vacuum performs VACUUM operation to reclaim unused space in the database.
// This should be called periodically during low-activity periods to prevent
// database file growth from INSERT OR REPLACE operations.
func (db *IndexDB) Vacuum() error {
	startTime := time.Now()
	_, err := db.Exec("VACUUM")
	if err != nil {
		logger.Errorf("[DB_MAINTENANCE] VACUUM failed: %v", err)
		return fmt.Errorf("failed to vacuum database: %w", err)
	}
	duration := time.Since(startTime)
	logger.Debugf("[DB_MAINTENANCE] VACUUM completed in %v", duration)
	return nil
}

func (db *IndexDB) DeleteStaleEntries(source string, pathPrefix string, scanStartTime int64) (int, error) {
	query := `
	DELETE FROM index_items 
	WHERE source = ? 
	AND path GLOB ? 
	AND last_updated < ?
	`

	globPattern := pathPrefix + "*"

	result, err := db.Exec(query, source, globPattern, scanStartTime)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_MAINTENANCE] DeleteStaleEntries: DB busy, skipping cleanup")
			return 0, nil
		}
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

func (db *IndexDB) GetRecursiveCount(source string, pathPrefix string) (dirs uint64, files uint64, error error) {
	query := `
	SELECT 
		SUM(CASE WHEN is_dir = 1 THEN 1 ELSE 0 END) as dir_count,
		SUM(CASE WHEN is_dir = 0 THEN 1 ELSE 0 END) as file_count
	FROM index_items 
	WHERE source = ? AND path GLOB ?
	`

	globPattern := pathPrefix + "*"

	var dirCount, fileCount sql.NullInt64
	err := db.QueryRow(query, source, globPattern).Scan(&dirCount, &fileCount)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	return uint64(dirCount.Int64), uint64(fileCount.Int64), nil
}

func (db *IndexDB) UpdateDirectorySize(source string, path string, newSize int64) error {
	query := `UPDATE index_items SET size = ? WHERE source = ? AND path = ?`
	_, err := db.Exec(query, newSize, source, path)
	if err != nil && !isBusyError(err) && !isTransactionError(err) {
		logger.Errorf("UpdateDirectorySize failed for source=%s path=%s: %v", source, path, err)
	}
	return err
}

// GetTypeGroupsForSize queries for file types that have 2+ files with the same size.
// This is used to filter down candidates before loading full file data for fuzzy matching.
// Returns file types (extensions) that have duplicates for the given size.
// Returns empty slice on database busy/lock errors (non-fatal).
func (db *IndexDB) GetTypeGroupsForSize(source string, size int64, pathPrefix string) ([]string, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT type, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size = ? AND is_dir = 0 AND path GLOB ?
		GROUP BY type
		HAVING COUNT(*) >= 2
		`
		args = []interface{}{source, size, pathPrefix + "*"}
	} else {
		query = `
		SELECT type, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size = ? AND is_dir = 0
		GROUP BY type
		HAVING COUNT(*) >= 2
		`
		args = []interface{}{source, size}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty results
		if isBusyError(err) || isTransactionError(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var types []string
	for rows.Next() {
		var fileType string
		var count int
		if err := rows.Scan(&fileType, &count); err != nil {
			return nil, err
		}
		types = append(types, fileType)
	}

	return types, rows.Err()
}

// GetFilesForMultipleSizes retrieves ALL files for a batch of sizes in a single query.
// This is much more efficient than querying each size individually.
// Returns a map of size -> files for easy grouping.
// Returns empty map on database busy/lock errors (non-fatal).
func (db *IndexDB) GetFilesForMultipleSizes(source string, sizes []int64, pathPrefix string) (map[int64][]*iteminfo.FileInfo, error) {
	if len(sizes) == 0 {
		return make(map[int64][]*iteminfo.FileInfo), nil
	}

	// Build IN clause with placeholders
	placeholders := make([]string, len(sizes))
	args := []interface{}{source}
	for i, size := range sizes {
		placeholders[i] = "?"
		args = append(args, size)
	}

	var query string
	if pathPrefix != "" {
		query = fmt.Sprintf(`
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size IN (%s) AND is_dir = 0 AND path GLOB ?
		ORDER BY size, type, name
		`, strings.Join(placeholders, ","))
		args = append(args, pathPrefix+"*")
	} else {
		query = fmt.Sprintf(`
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size IN (%s) AND is_dir = 0
		ORDER BY size, type, name
		`, strings.Join(placeholders, ","))
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty results
		if isBusyError(err) || isTransactionError(err) {
			return make(map[int64][]*iteminfo.FileInfo), nil
		}
		return nil, err
	}
	defer rows.Close()

	// Group files by size
	filesBySize := make(map[int64][]*iteminfo.FileInfo)
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		filesBySize[item.Size] = append(filesBySize[item.Size], item)
	}

	return filesBySize, rows.Err()
}

// GetFilesBySizeAndType retrieves files matching both size and type.
// This loads a smaller subset than GetFilesBySize, allowing fuzzy filename matching in memory.
// Returns empty slice on database busy/lock errors (non-fatal).
func (db *IndexDB) GetFilesBySizeAndType(source string, size int64, fileType string, pathPrefix string) ([]*iteminfo.FileInfo, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND type = ? AND is_dir = 0 AND path GLOB ?
		ORDER BY name
		`
		args = []interface{}{source, size, fileType, pathPrefix + "*"}
	} else {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND type = ? AND is_dir = 0
		ORDER BY name
		`
		args = []interface{}{source, size, fileType}
	}

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty results
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

// GetFilesBySize retrieves all files with a specific size for a source, optionally filtered by path prefix.
// Used for duplicate detection - returns files ordered by name for efficient grouping.
// Returns empty slice on database busy/lock errors (non-fatal).
// NOTE: For better performance when dealing with many files of the same size,
// consider using GetFilenameGroupsForSize + GetFilesBySizeAndName to filter at SQL level.
func (db *IndexDB) GetFilesBySize(source string, size int64, pathPrefix string) ([]*iteminfo.FileInfo, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND is_dir = 0 AND path GLOB ?
		ORDER BY name
		`
		args = []interface{}{source, size, pathPrefix + "*"}
	} else {
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND is_dir = 0
		ORDER BY name
		`
		args = []interface{}{source, size}
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

// GetSizeGroupsForDuplicates queries for all size groups that have 2+ files for a specific source.
// Returns sizes in descending order (largest first) and a count map.
// Optionally filters by path prefix for scoped searches.
// Returns empty results on database busy/lock errors (non-fatal).
func (db *IndexDB) GetSizeGroupsForDuplicates(source string, minSize int64, pathPrefix string) ([]int64, map[int64]int, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		query = `
		SELECT size, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size >= ? AND is_dir = 0 AND path GLOB ?
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
		`
		args = []interface{}{source, minSize, pathPrefix + "*"}
	} else {
		query = `
		SELECT size, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size >= ? AND is_dir = 0
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
		`
		args = []interface{}{source, minSize}
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

// GetTotalSize returns the sum of all file sizes in the index for a specific source (excluding directories).
// Returns 0 on database busy/lock errors (non-fatal).
func (db *IndexDB) GetTotalSize(source string) (uint64, error) {
	query := `SELECT COALESCE(SUM(size), 0) FROM index_items WHERE source = ? AND is_dir = 0`

	var totalSize int64
	err := db.QueryRow(query, source).Scan(&totalSize)
	if err != nil {
		// Soft failure: DB is busy or locked, return 0
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}

	return uint64(totalSize), nil
}

// RecalculateDirectorySizes recalculates and updates all directory sizes based on their children
// for a given path prefix. This should be called after bulk insertions to ensure directory sizes are accurate.
// Returns the number of directories updated.
func (db *IndexDB) RecalculateDirectorySizes(source, pathPrefix string) (int, error) {
	// Get all directories under the path prefix, ordered by depth (deepest first)
	// This ensures we calculate child directories before parent directories
	query := `
	SELECT path FROM index_items
	WHERE source = ? AND is_dir = 1 AND path GLOB ?
	ORDER BY LENGTH(path) - LENGTH(REPLACE(path, '/', '')) DESC
	`

	rows, err := db.Query(query, source, pathPrefix+"*")
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}
	defer rows.Close()

	var directories []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return 0, err
		}
		directories = append(directories, path)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// For each directory, calculate the sum of direct children sizes
	updateCount := 0
	for _, dirPath := range directories {
		// Calculate total size of all files (not directories) under this path
		// Only sum file sizes to avoid double-counting (directory sizes are derived)
		sizeQuery := `
		SELECT COALESCE(SUM(size), 0)
		FROM index_items
		WHERE source = ? AND path GLOB ? AND path != ? AND is_dir = 0
		`

		var totalSize int64
		err := db.QueryRow(sizeQuery, source, dirPath+"*", dirPath).Scan(&totalSize)
		if err != nil {
			if isBusyError(err) || isTransactionError(err) {
				continue
			}
			logger.Errorf("[DB_SIZE_CALC] Failed to calculate size for %s: %v", dirPath, err)
			continue
		}

		// Update the directory's size and last_updated timestamp
		// This prevents the directory from being deleted as stale during cleanup
		nowUnix := time.Now().Unix()
		updateQuery := `UPDATE index_items SET size = ?, last_updated = ? WHERE source = ? AND path = ?`
		result, err := db.Exec(updateQuery, totalSize, nowUnix, source, dirPath)
		if err != nil {
			if isBusyError(err) || isTransactionError(err) {
				continue
			}
			logger.Errorf("[DB_SIZE_CALC] Failed to update size for %s: %v", dirPath, err)
			continue
		}

		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			updateCount++
		}
	}

	return updateCount, nil
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
