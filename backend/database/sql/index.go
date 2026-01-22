package sql

import (
	"database/sql"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// IndexDB manages the SQLite database for the file index.
// It wraps the underlying sql.DB connection and provides type-safe methods.
type IndexDB struct {
	*TempDB
}

func createIndexDB(name string, journalMode string, lockingMode string, batchSize int, cacheSizeMB int, disableReuse bool) (*IndexDB, error) {
	var persistentFile string
	if !disableReuse {
		// Reuse enabled (default): use persistent file
		persistentFile = fmt.Sprintf("index_%s.db", name)
	}
	// If disableReuse is true, persistentFile stays empty and a temp file will be created

	// Determine locking mode based on journal mode
	if lockingMode == "" {
		if journalMode == "WAL" {
			lockingMode = "NORMAL"
		} else {
			lockingMode = "EXCLUSIVE"
		}
	}

	// Convert MB to KB for SQLite cache_size
	cacheSizeKB := cacheSizeMB * 1024 // Positive value = KB

	db, err := NewTempDB("index_"+name, &TempDBConfig{
		BatchSize:           batchSize,
		CacheSizeKB:         cacheSizeKB,      // From config, converted to KB
		SoftHeapLimitBytes:  16 * 1024 * 1024, // 16MB soft heap limit (reduced to minimize memory pressure)
		CacheSpillThreshold: 2000,             // Spill dirty pages to disk when cache exceeds 2000 pages (~8MB)
		MmapSize:            0,                // Disable mmap to prevent additional OS page cache usage
		Synchronous:         "OFF",            // No sync for maximum performance - safe since DB can be rebuilt
		TempStore:           "FILE",           // FILE instead of MEMORY to reduce memory usage
		JournalMode:         journalMode,      // Configurable journal mode
		LockingMode:         lockingMode,      // Configurable locking mode
		PersistentFile:      persistentFile,   // Use fixed filename or empty for temp file
	})
	if err != nil {
		return nil, err
	}
	return &IndexDB{TempDB: db}, nil
}

// NewIndexDB creates a new index database in the cache directory.
// Uses a fixed filename for persistence (index_{name}.db) unless disableReuse is true.
// journalMode: "OFF", "WAL", "DELETE" - controls SQLite journal mode
// batchSize: number of items per batch transaction
// cacheSizeMB: cache size in megabytes
// disableReuse: true to delete and recreate DB on startup, false to reuse existing (default)
// Returns the IndexDB and a boolean indicating if the database was recreated (true) or reused (false)
func NewIndexDB(name string, journalMode string, batchSize int, cacheSizeMB int, disableReuse bool) (*IndexDB, bool, error) {
	wasRecreated := disableReuse // If disableReuse is true, we're creating fresh

	idxDB, err := createIndexDB(name, journalMode, "", batchSize, cacheSizeMB, disableReuse)
	if err != nil {
		return nil, false, err
	}

	// For persistent databases, check integrity on startup
	// If corrupted, delete and recreate the database
	if !disableReuse {
		if err := idxDB.checkIntegrityAndRecreateIfNeeded(); err != nil {
			// Database was recreated due to corruption
			wasRecreated = true
			idxDB.Close()

			// Recreate the database connection
			idxDB, err = createIndexDB(name, journalMode, "", batchSize, cacheSizeMB, disableReuse)
			if err != nil {
				return nil, false, err
			}
		}
	}

	if err := idxDB.CreateIndexTable(); err != nil {
		idxDB.Close()
		return nil, false, err
	}
	go idxDB.startPeriodicCleanup()
	return idxDB, wasRecreated, nil
}

// checkIntegrityAndRecreateIfNeeded checks database integrity.
// If corruption is detected, deletes the database file and returns an error to trigger recreation.
// Returns nil if integrity is OK, error if database needs to be recreated.
func (db *IndexDB) checkIntegrityAndRecreateIfNeeded() error {
	// Quick integrity check
	var result string
	err := db.QueryRow("PRAGMA quick_check").Scan(&result)
	if err != nil {
		// If we can't even run the check, assume corruption
		logger.Warningf("[DB_INTEGRITY] Cannot run integrity check: %v", err)
		return db.deleteCorruptedDatabase()
	}

	if result != "ok" {
		logger.Warningf("[DB_INTEGRITY] Database integrity check failed, database will be recreated: %s", result)
		return db.deleteCorruptedDatabase()
	}
	return nil
}

// deleteCorruptedDatabase deletes the corrupted database file and associated files.
func (db *IndexDB) deleteCorruptedDatabase() error {
	dbPath := db.Path()
	if dbPath == "" {
		return fmt.Errorf("database path is empty")
	}

	// Close the database connection first
	db.Close()

	// Delete the main database file
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete corrupted database: %w", err)
	}

	// Delete any associated WAL/shm files
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	return fmt.Errorf("database was corrupted and deleted - will be recreated")
}

// startPeriodicCleanup starts a background goroutine that cleans up stale database entries every 24 hours.
// Stale entries are items where last_updated is older than 24 hours.
func (db *IndexDB) startPeriodicCleanup() {
	cleanupInterval := 24 * time.Hour
	ticker := time.NewTicker(cleanupInterval)
	time.Sleep(cleanupInterval)
	for range ticker.C {
		logger.Infof("[DB_MAINTENANCE] Starting periodic cleanup of stale index entries (older than 24 hours)")
		deletedCount, err := db.DeleteStaleItemsOlderThan(cleanupInterval)
		if err != nil {
			logger.Errorf("[DB_MAINTENANCE] Failed to cleanup stale entries: %v", err)
		} else if deletedCount > 0 {
			logger.Infof("[DB_MAINTENANCE] Cleaned up %d stale index entries", deletedCount)
		}
	}
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
	
	CREATE INDEX IF NOT EXISTS idx_source_parent_path ON index_items(source, parent_path);
	CREATE INDEX IF NOT EXISTS idx_source_size ON index_items(source, size);
	CREATE INDEX IF NOT EXISTS idx_last_updated ON index_items(source, last_updated);
	`
	_, err := db.Exec(query)
	return err
}

func (db *IndexDB) InsertItem(source, path string, info *iteminfo.FileInfo) error {
	query := `
	INSERT INTO index_items (source, path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(source, path) DO UPDATE SET
		parent_path = excluded.parent_path,
		name = excluded.name,
		size = excluded.size,
		mod_time = excluded.mod_time,
		type = excluded.type,
		is_dir = excluded.is_dir,
		is_hidden = excluded.is_hidden,
		has_preview = excluded.has_preview,
		last_updated = excluded.last_updated
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

	startTime := time.Now()

	tx, err := db.BeginTransaction()
	if err != nil {
		// Soft failure: DB is busy or locked, skip this update
		if isBusyError(err) || isTransactionError(err) {
			db.mu.Unlock() // Release mutex on error
			logger.Debugf("[DB_TX] BulkInsertItems: BeginTransaction failed (DB busy/locked), skipping - took %v", time.Since(startTime))
			return nil // Non-fatal: filesystem will be used as fallback
		}
		db.mu.Unlock() // Release mutex on error
		return err     // Hard failure: something is wrong with the DB
	}
	defer db.mu.Unlock() // Ensure mutex is always released

	// Always update all fields including last_updated, even if nothing changed
	// This ensures last_updated stays current for stale entry detection
	stmt, err := tx.Prepare(`
	INSERT INTO index_items (source, path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview, last_updated)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(source, path) DO UPDATE SET
		parent_path = excluded.parent_path,
		name = excluded.name,
		size = excluded.size,
		mod_time = excluded.mod_time,
		type = excluded.type,
		is_dir = excluded.is_dir,
		is_hidden = excluded.is_hidden,
		has_preview = excluded.has_preview,
		last_updated = excluded.last_updated
	`)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			// With EXCLUSIVE locking and mutex, this shouldn't happen.
			// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
			logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Prepare: %v", err)
			return nil
		}
		return err
	}
	defer stmt.Close() // Ensure statement is always closed

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
			if isBusyError(err) || isTransactionError(err) {
				// With EXCLUSIVE locking and mutex, this shouldn't happen.
				// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
				logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Exec: %v", err)
				return nil
			}
			logger.Errorf("[DB] BulkInsertItems: Exec failed for path=%s, source=%s, error=%v", info.Path, source, err)
			return err
		}
	}

	// Try to commit
	if err := tx.Commit(); err != nil {
		if isBusyError(err) || isTransactionError(err) {
			// With EXCLUSIVE locking and mutex, this shouldn't happen.
			// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
			logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Commit: %v", err)
			return nil
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

// GetHasPreviewBatch retrieves hasPreview status for multiple directory paths in a single query.
// This is optimized for the N+1 query pattern in GetDirInfo where we need hasPreview for all subdirectories.
// Returns map[path]hasPreview. Missing paths in the result indicate they're not in the database yet.
func (db *IndexDB) GetHasPreviewBatch(source string, paths []string) (map[string]bool, error) {
	if len(paths) == 0 {
		return make(map[string]bool), nil
	}

	// Build query with IN clause - only select path and has_preview for efficiency
	placeholders := make([]string, len(paths))
	args := make([]interface{}, len(paths)+1)
	args[0] = source
	for i, path := range paths {
		placeholders[i] = "?"
		args[i+1] = path
	}

	query := fmt.Sprintf(`
	SELECT path, has_preview
	FROM index_items WHERE source = ? AND path IN (%s) AND is_dir = 1
	`, strings.Join(placeholders, ","))

	rows, err := db.Query(query, args...)
	if err != nil {
		// Soft failure: DB is busy or locked, return empty map
		if isBusyError(err) || isTransactionError(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]bool)
	for rows.Next() {
		var path string
		var hasPreview bool
		if err := rows.Scan(&path, &hasPreview); err != nil {
			return nil, err
		}
		result[path] = hasPreview
	}

	return result, nil
}

// GetDirectoryChildren retrieves all children of a directory for a specific source.
func (db *IndexDB) GetDirectoryChildren(source, dirPath string) ([]*iteminfo.FileInfo, error) {
	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE source = ? AND parent_path = ?
	ORDER BY is_dir DESC, name ASC
	`

	rows, err := db.Query(query, source, dirPath)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			logger.Warningf("[DB_TX] GetDirectoryChildren: DB busy/locked, skipping query")
			return []*iteminfo.FileInfo{}, nil
		}
		logger.Errorf("GetDirectoryChildren: Query failed for source=%s, parent_path=%s, error=%v", source, dirPath, err)
		return nil, err
	}
	defer rows.Close()

	var children []*iteminfo.FileInfo
	for rows.Next() {
		item, err := scanRow(rows)
		if err != nil {
			logger.Errorf("GetDirectoryChildren: scanRow failed, error=%v", err)
			return nil, err
		}
		children = append(children, item)
	}
	return children, nil
}

// DeleteItem removes an item and optionally its children (recursive) for a specific source.
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

	// Delete children - use range query on PRIMARY KEY (source, path) for optimal index utilization
	nextPrefix := getNextPathPrefix(dirPrefix)
	_, err := db.Exec("DELETE FROM index_items WHERE source = ? AND path >= ? AND path < ?", source, dirPrefix, nextPrefix)
	return err
}

// helps prevent SQLite page cache memory leaks by shrinking memory
func (db *IndexDB) ShrinkMemory() error {
	_, err := db.Exec("PRAGMA shrink_memory")
	return err
}

// Optimize runs PRAGMA optimize to update query planner statistics.
// This helps SQLite choose more efficient query plans and should be called periodically.
func (db *IndexDB) Optimize() error {
	_, err := db.Exec("PRAGMA optimize")
	return err
}

// ExplainQueryPlan analyzes a query and returns the execution plan as a string.
// This is useful for debugging and optimizing queries to reduce page cache usage.
func (db *IndexDB) ExplainQueryPlan(query string, args ...interface{}) (string, error) {
	explainQuery := "EXPLAIN QUERY PLAN " + query
	rows, err := db.Query(explainQuery, args...)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		if err := rows.Scan(&id, &parent, &notused, &detail); err != nil {
			return "", err
		}
		plan.WriteString(fmt.Sprintf("%d|%d|%s\n", id, parent, detail))
	}
	return plan.String(), rows.Err()
}

// UpdateCacheSize updates the SQLite cache size at runtime.
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

// DeleteStaleEntriesQuick handles cleanup for quick scans (two-phase approach)
// Phase 1: Delete folders that weren't updated (they were deleted from filesystem)
// Phase 2: Delete files only in folders that had modtime changes
// DeleteStaleFolders deletes folders that weren't updated during a quick scan
func (db *IndexDB) DeleteStaleFolders(source string, pathPrefix string, scanStartTime int64, root bool) (int, error) {
	if scanStartTime == 0 {
		return 0, nil
	}

	const batchSize = 1000
	totalDeleted := 0

	// Delete stale folders (weren't touched during quick scan)
	for {
		var query string
		var result sql.Result
		var err error

		if root {
			// Root scanner: only check direct children folders
			query = `
			DELETE FROM index_items 
			WHERE rowid IN (
				SELECT rowid FROM index_items
				WHERE source = ? 
				AND parent_path = '/'
				AND is_dir = 1
				AND last_updated < ?
				LIMIT ?
			)
			`
			result, err = db.Exec(query, source, scanStartTime, batchSize)
		} else {
			// Child scanner: check all folders under pathPrefix
			nextPrefix := getNextPathPrefix(pathPrefix)
			query = `
			DELETE FROM index_items 
			WHERE rowid IN (
				SELECT rowid FROM index_items
				WHERE source = ? 
				AND path >= ? AND path < ?
				AND is_dir = 1
				AND last_updated < ?
				LIMIT ?
			)
			`
			result, err = db.Exec(query, source, pathPrefix, nextPrefix, scanStartTime, batchSize)
		}

		if err != nil {
			if isBusyError(err) || isTransactionError(err) {
				logger.Debugf("[DB_MAINTENANCE] DeleteStaleEntriesQuick Phase 1: DB busy, skipping cleanup")
				return totalDeleted, nil
			}
			return totalDeleted, err
		}

		rowsAffected, _ := result.RowsAffected()
		totalDeleted += int(rowsAffected)

		if rowsAffected < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

// DeleteStaleFilesInDirs deletes files that weren't updated in specific directories
// Uses batched IN clauses for efficiency with potentially large directory lists
func (db *IndexDB) DeleteStaleFilesInDirs(source string, updatedDirs []string, scanStartTime int64) (int, error) {
	if scanStartTime == 0 || len(updatedDirs) == 0 {
		return 0, nil
	}

	const maxPathsPerBatch = 500 // SQLite handles IN clauses well up to ~1000, we use 500 for safety
	const deleteLimit = 1000     // Limit per DELETE to prevent memory spikes
	totalDeleted := 0

	// Process directories in batches
	for batchStart := 0; batchStart < len(updatedDirs); batchStart += maxPathsPerBatch {
		batchEnd := batchStart + maxPathsPerBatch
		if batchEnd > len(updatedDirs) {
			batchEnd = len(updatedDirs)
		}
		batch := updatedDirs[batchStart:batchEnd]

		// Build placeholders for IN clause
		placeholders := make([]string, len(batch))
		args := make([]interface{}, len(batch)+2)
		args[0] = source
		args[1] = scanStartTime
		for i, dir := range batch {
			placeholders[i] = "?"
			args[i+2] = dir
		}
		placeholdersStr := strings.Join(placeholders, ",")

		// Delete files in batches to prevent memory spikes
		for {
			query := fmt.Sprintf(`
				DELETE FROM index_items 
				WHERE rowid IN (
					SELECT rowid FROM index_items
					WHERE source = ?
					AND is_dir = 0
					AND last_updated < ?
					AND parent_path IN (%s)
					LIMIT ?
				)
			`, placeholdersStr)

			// Append limit to args
			queryArgs := append(args, deleteLimit)

			result, err := db.Exec(query, queryArgs...)
			if err != nil {
				if isBusyError(err) || isTransactionError(err) {
					logger.Debugf("[DB_MAINTENANCE] DeleteStaleFilesInDirs: DB busy, skipping batch")
					break // Skip this batch and continue with next
				}
				return totalDeleted, err
			}

			rowsAffected, _ := result.RowsAffected()
			totalDeleted += int(rowsAffected)

			if rowsAffected < deleteLimit {
				break // No more files to delete in this batch
			}
		}
	}

	return totalDeleted, nil
}

func (db *IndexDB) DeleteStaleEntries(source string, pathPrefix string, scanStartTime int64, root bool) (int, error) {
	if scanStartTime == 0 {
		return 0, nil
	}
	// Delete in batches to prevent memory spikes from large transactions
	const batchSize = 1000
	totalDeleted := 0

	for {
		var query string
		var result sql.Result
		var err error

		if root {
			// Root scanner: only delete items whose parent_path is "/" (direct children)
			// This prevents root scanner from deleting items managed by child scanners
			query = `
			DELETE FROM index_items 
			WHERE rowid IN (
				SELECT rowid FROM index_items
				WHERE source = ? 
				AND parent_path = '/'
				AND last_updated < ?
				LIMIT ?
			)
			`
			result, err = db.Exec(query, source, scanStartTime, batchSize)
		} else {
			// Child scanner: delete everything under this path prefix recursively
			// Use range query on PRIMARY KEY (source, path) for optimal index utilization
			nextPrefix := getNextPathPrefix(pathPrefix)
			query = `
			DELETE FROM index_items 
			WHERE rowid IN (
				SELECT rowid FROM index_items
				WHERE source = ? 
				AND path >= ? AND path < ?
				AND last_updated < ?
				LIMIT ?
			)
			`
			result, err = db.Exec(query, source, pathPrefix, nextPrefix, scanStartTime, batchSize)
		}

		if err != nil {
			if isBusyError(err) || isTransactionError(err) {
				logger.Debugf("[DB_MAINTENANCE] DeleteStaleEntries: DB busy, skipping cleanup")
				return totalDeleted, nil
			}
			return totalDeleted, err
		}

		rowsAffected, _ := result.RowsAffected()
		totalDeleted += int(rowsAffected)

		// If we deleted fewer than batchSize rows, we're done
		if rowsAffected < batchSize {
			break
		}
	}

	return totalDeleted, nil
}

// DeleteStaleItemsOlderThan deletes all items across all sources where last_updated is older than the specified duration.
// This is used for periodic cleanup of stale database entries.
func (db *IndexDB) DeleteStaleItemsOlderThan(olderThan time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-olderThan).Unix()
	query := `
	DELETE FROM index_items 
	WHERE last_updated < ?
	`

	result, err := db.Exec(query, cutoffTime)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_MAINTENANCE] DeleteStaleItemsOlderThan: DB busy, skipping cleanup")
			return 0, nil
		}
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return int(rowsAffected), nil
}

// GetRecursiveCount counts directories and files recursively for a child scanner
func (db *IndexDB) GetRecursiveCount(source string, pathPrefix string) (dirs uint64, files uint64, err error) {
	// Child scanner: count both directories and files recursively
	// Use range query on PRIMARY KEY (source, path) for optimal index utilization
	nextPrefix := getNextPathPrefix(pathPrefix)
	query := `
	SELECT 
		SUM(CASE WHEN is_dir = 1 THEN 1 ELSE 0 END) as dir_count,
		SUM(CASE WHEN is_dir = 0 THEN 1 ELSE 0 END) as file_count
	FROM index_items 
	WHERE source = ? AND path >= ? AND path < ?
	`

	var dirCount, fileCount sql.NullInt64
	err = db.QueryRow(query, source, pathPrefix, nextPrefix).Scan(&dirCount, &fileCount)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, 0, nil
		}
		return 0, 0, err
	}

	return uint64(dirCount.Int64), uint64(fileCount.Int64), nil
}

// GetDirectFileCount counts only files directly under the given path (non-recursive)
// For root scanner ("/"), this counts files like "/filename" but excludes "/subdir/filename"
func (db *IndexDB) GetDirectFileCount(source string, pathPrefix string) (files uint64, err error) {
	// Use parent_path for direct children instead of pattern matching
	// This is more efficient as it can use the idx_source_parent_path index directly
	query := `
	SELECT 
		COUNT(*) as file_count
	FROM index_items 
	WHERE source = ? AND parent_path = ? AND is_dir = 0
	`

	var fileCount sql.NullInt64
	err = db.QueryRow(query, source, pathPrefix).Scan(&fileCount)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}

	return uint64(fileCount.Int64), nil
}

// GetTypeGroupsForSize queries for file types that have 2+ files with the same size.
func (db *IndexDB) GetTypeGroupsForSize(source string, size int64, pathPrefix string) ([]string, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		nextPrefix := getNextPathPrefix(pathPrefix)
		query = `
		SELECT type, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size = ? AND is_dir = 0 AND path >= ? AND path < ?
		GROUP BY type
		HAVING COUNT(*) >= 2
		`
		args = []interface{}{source, size, pathPrefix, nextPrefix}
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
		nextPrefix := getNextPathPrefix(pathPrefix)
		query = fmt.Sprintf(`
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size IN (%s) AND is_dir = 0 AND path >= ? AND path < ?
		ORDER BY size, type, name
		`, strings.Join(placeholders, ","))
		args = append(args, pathPrefix, nextPrefix)
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

// GetAllDirectories returns all directory paths for a source (used for size recalculation)
func (db *IndexDB) GetAllDirectories(source string) ([]string, error) {
	query := `
		SELECT path
		FROM index_items
		WHERE source = ? AND is_dir = 1
		ORDER BY path
	`

	rows, err := db.Query(query, source)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var directories []string
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			return nil, err
		}
		directories = append(directories, path)
	}

	return directories, rows.Err()
}

// UpdateFolderSizesIfChanged updates ONLY the size field for folders where the size actually changed
// This is a single SQL operation that filters unchanged rows in the WHERE clause
// Minimizes transaction footprint by only touching rows that need updating
func (db *IndexDB) UpdateFolderSizesIfChanged(source string, pathSizes map[string]uint64) (int64, error) {
	if len(pathSizes) == 0 {
		return 0, nil
	}

	// Build CASE statements for both SET and WHERE clauses
	var setCaseParts []string
	var whereCaseParts []string
	var args []interface{}
	var paths []string
	var placeholders []string

	for path, size := range pathSizes {
		setCaseParts = append(setCaseParts, "WHEN path = ? THEN ?")
		whereCaseParts = append(whereCaseParts, "WHEN path = ? THEN ?")
		args = append(args, path, size)
		args = append(args, path, size) // Duplicate for WHERE clause
		paths = append(paths, path)
		placeholders = append(placeholders, "?")
	}

	// Single UPDATE that only touches rows where size differs
	// The WHERE clause filters out unchanged rows, minimizing transaction size
	query := fmt.Sprintf(`
		UPDATE index_items 
		SET size = CASE %s END
		WHERE source = ? 
		  AND path IN (%s) 
		  AND is_dir = 1
		  AND size != CASE %s END
	`, strings.Join(setCaseParts, " "), strings.Join(placeholders, ","), strings.Join(whereCaseParts, " "))

	args = append(args, source)
	for _, path := range paths {
		args = append(args, path)
	}

	result, err := db.Exec(query, args...)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_TX] UpdateFolderSizesIfChanged: DB busy/locked, skipping update")
			return 0, nil // Non-fatal: sizes will be synced on next scan
		}
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	return rowsAffected, nil
}

// LoadFolderSizes loads all folder sizes for a source from the database
// Used on initialization to populate the in-memory map with existing sizes
func (db *IndexDB) LoadFolderSizes(source string) (map[string]uint64, error) {
	query := `
		SELECT path, size 
		FROM index_items 
		WHERE source = ? AND is_dir = 1
	`

	rows, err := db.Query(query, source)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	folderSizes := make(map[string]uint64)
	for rows.Next() {
		var path string
		var size uint64
		if err := rows.Scan(&path, &size); err != nil {
			return nil, err
		}
		folderSizes[path] = size
	}

	return folderSizes, rows.Err()
}

// GetFilesBySizeAndType retrieves files matching both size and type.
func (db *IndexDB) GetFilesBySizeAndType(source string, size int64, fileType string, pathPrefix string) ([]*iteminfo.FileInfo, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		nextPrefix := getNextPathPrefix(pathPrefix)
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND type = ? AND is_dir = 0 AND path >= ? AND path < ?
		ORDER BY name
		`
		args = []interface{}{source, size, fileType, pathPrefix, nextPrefix}
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
func (db *IndexDB) GetFilesBySize(source string, size int64, pathPrefix string) ([]*iteminfo.FileInfo, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		nextPrefix := getNextPathPrefix(pathPrefix)
		query = `
		SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
		FROM index_items 
		WHERE source = ? AND size = ? AND is_dir = 0 AND path >= ? AND path < ?
		ORDER BY name
		`
		args = []interface{}{source, size, pathPrefix, nextPrefix}
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
func (db *IndexDB) GetSizeGroupsForDuplicates(source string, minSize int64, pathPrefix string) ([]int64, map[int64]int, error) {
	var query string
	var args []interface{}

	if pathPrefix != "" {
		nextPrefix := getNextPathPrefix(pathPrefix)
		query = `
		SELECT size, COUNT(*) as count
		FROM index_items
		WHERE source = ? AND size >= ? AND is_dir = 0 AND path >= ? AND path < ?
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
		`
		args = []interface{}{source, minSize, pathPrefix, nextPrefix}
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

// Helper functions

// getNextPathPrefix returns the lexicographically next string after the given prefix.
// This is used for efficient range queries on path columns with PRIMARY KEY (source, path).
// For example: "/lost/" -> "/lost0" (which is > "/lost/" but < "/lost2")
// This allows us to use range queries (path >= prefix AND path < nextPrefix) instead of LIKE/GLOB.
func getNextPathPrefix(prefix string) string {
	if prefix == "" {
		return "\x00" // Return null character for empty prefix
	}
	// Find the lexicographically next string by incrementing the last character
	// If last char is not the max, increment it; otherwise append a character
	runes := []rune(prefix)
	lastIdx := len(runes) - 1

	// Try to increment the last character
	if lastIdx >= 0 {
		lastChar := runes[lastIdx]
		// If it's not the maximum character, increment it
		if lastChar < 0x10FFFF { // Max Unicode code point
			runes[lastIdx] = lastChar + 1
			return string(runes)
		}
	}
	// If we can't increment, append a character (use 0x00 which sorts before most characters)
	return prefix + "\x00"
}

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
