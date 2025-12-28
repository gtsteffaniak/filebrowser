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
type IndexDB struct {
	*TempDB
}

// NewIndexDB creates a new index database in the cache directory.
func NewIndexDB(name string) (*IndexDB, error) {
	db, err := NewTempDB("index_"+name, &TempDBConfig{
		BatchSize:           1000,             // 1000 items per batch
		CacheSizeKB:         -250,             // 1MB cache (250 pages * 4KB = 1MB) - reduced to minimize OS page cache pressure
		SoftHeapLimitBytes:  16 * 1024 * 1024, // 16MB soft heap limit (reduced to minimize memory pressure)
		CacheSpillThreshold: 200,              // Spill dirty pages to disk when cache exceeds 200 pages (~800KB) - more aggressive
		MmapSize:            0,                // Disable mmap to prevent additional OS page cache usage
		Synchronous:         "OFF",            // OFF for maximum write performance - data integrity not critical
		TempStore:           "FILE",           // FILE instead of MEMORY to reduce memory usage
		JournalMode:         "OFF",            // OFF for maximum write performance - data integrity not critical
		LockingMode:         "EXCLUSIVE",      // EXCLUSIVE mode - better cache retention, no change counter overhead
	})
	if err != nil {
		return nil, err
	}
	idxDB := &IndexDB{TempDB: db}
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
	CREATE INDEX IF NOT EXISTS idx_source_parent_path_is_dir_name ON index_items(source, parent_path, is_dir, name);
	CREATE INDEX IF NOT EXISTS idx_source_size ON index_items(source, size);
	CREATE INDEX IF NOT EXISTS idx_source_size_type ON index_items(source, size, type);
	CREATE INDEX IF NOT EXISTS idx_name ON index_items(name);
	CREATE INDEX IF NOT EXISTS idx_last_updated ON index_items(source, last_updated);
	CREATE INDEX IF NOT EXISTS idx_source_is_dir ON index_items(source, is_dir);
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
	WHERE excluded.last_updated > index_items.last_updated OR
	      excluded.size != index_items.size OR
	      excluded.mod_time != index_items.mod_time OR
	      excluded.has_preview != index_items.has_preview OR
	      excluded.parent_path != index_items.parent_path OR
	      excluded.name != index_items.name OR
	      excluded.type != index_items.type OR
	      excluded.is_dir != index_items.is_dir OR
	      excluded.is_hidden != index_items.is_hidden
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
			logger.Debugf("[DB_TX] BulkInsertItems: BeginTransaction failed (DB busy/locked), skipping - took %v", time.Since(startTime))
			return nil // Non-fatal: filesystem will be used as fallback
		}
		return err // Hard failure: something is wrong with the DB
	}

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
	WHERE excluded.last_updated > index_items.last_updated OR
	      excluded.size != index_items.size OR
	      excluded.mod_time != index_items.mod_time OR
	      excluded.has_preview != index_items.has_preview OR
	      excluded.parent_path != index_items.parent_path OR
	      excluded.name != index_items.name OR
	      excluded.type != index_items.type OR
	      excluded.is_dir != index_items.is_dir OR
	      excluded.is_hidden != index_items.is_hidden
	`)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			// With EXCLUSIVE locking and single connection, this shouldn't happen.
			// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
			logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Prepare (EXCLUSIVE mode): %v", err)
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
				// With EXCLUSIVE locking and single connection, this shouldn't happen.
				// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
				logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Exec (EXCLUSIVE mode): %v", err)
				return nil
			}
			return err
		}
	}

	// Try to commit
	if err := tx.Commit(); err != nil {
		if isBusyError(err) || isTransactionError(err) {
			// With EXCLUSIVE locking and single connection, this shouldn't happen.
			// Log as warning to surface potential issues (another process accessing DB, bug, etc.)
			logger.Errorf("[DB] BulkInsertItems: Unexpected busy/lock error during Commit (EXCLUSIVE mode): %v", err)
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

// BulkUpdateSizes updates the sizes of multiple items in a single query for a specific source.
func (db *IndexDB) BulkUpdateSizes(source string, pathSizeUpdates map[string]int64) error {
	if len(pathSizeUpdates) == 0 {
		return nil
	}
	var caseParts []string
	var args []interface{}
	var paths []string
	var placeholders []string

	nowUnix := time.Now().Unix()

	for path, sizeDelta := range pathSizeUpdates {
		if sizeDelta == 0 {
			continue // Skip zero deltas - no change needed
		}
		caseParts = append(caseParts, "WHEN path = ? THEN ?")
		args = append(args, path, sizeDelta)
		paths = append(paths, path)
		placeholders = append(placeholders, "?")
	}

	if len(caseParts) == 0 {
		return nil // All deltas were zero
	}

	query := fmt.Sprintf(`
		UPDATE index_items 
		SET size = size + CASE %s END,
		    last_updated = ?
		WHERE source = ? AND path IN (%s)
	`, strings.Join(caseParts, " "), strings.Join(placeholders, ","))

	args = append(args, nowUnix)
	args = append(args, source)
	for _, path := range paths {
		args = append(args, path)
	}
	_, err := db.Exec(query, args...)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			logger.Debugf("[DB_TX] BulkUpdateSizes: DB busy/locked, skipping update")
			return nil // Non-fatal: sizes can be recalculated later
		}
		return err
	}
	return nil
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

func (db *IndexDB) DeleteStaleEntries(source string, pathPrefix string, scanStartTime int64) (int, error) {
	// Use range query on PRIMARY KEY (source, path) for optimal index utilization
	// Range queries (path >= ? AND path < ?) are MORE efficient than GLOB/LIKE because:
	// 1. They can use the PRIMARY KEY index directly without pattern matching
	// 2. SQLite can optimize range queries better than pattern matching
	// 3. No pattern evaluation overhead - just direct index seeks
	// This is better than GLOB 'prefix*' which still requires pattern evaluation
	nextPrefix := getNextPathPrefix(pathPrefix)
	query := `
	DELETE FROM index_items 
	WHERE source = ? 
	AND path >= ? AND path < ?
	AND last_updated < ?
	`

	result, err := db.Exec(query, source, pathPrefix, nextPrefix, scanStartTime)
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

func (db *IndexDB) UpdateDirectorySize(source string, path string, newSize int64) error {
	query := `UPDATE index_items SET size = ? WHERE source = ? AND path = ?`
	_, err := db.Exec(query, newSize, source, path)
	if err != nil && !isBusyError(err) && !isTransactionError(err) {
		logger.Errorf("UpdateDirectorySize failed for source=%s path=%s: %v", source, path, err)
	}
	return err
}

// UpdateDirectorySizeIfStale updates directory size only if the directory wasn't modified during the scan.
func (db *IndexDB) UpdateDirectorySizeIfStale(source string, path string, newSize int64, scanStartTime int64) (bool, error) {
	if scanStartTime == 0 {
		// No scan start time provided, always update (backward compatibility)
		return true, db.UpdateDirectorySize(source, path, newSize)
	}

	// Only update if last_updated < scanStartTime (directory wasn't modified during scan)
	query := `UPDATE index_items 
		SET size = ?, last_updated = ? 
		WHERE source = ? AND path = ? AND last_updated < ?`
	nowUnix := time.Now().Unix()
	result, err := db.Exec(query, newSize, nowUnix, source, path, scanStartTime)
	if err != nil && !isBusyError(err) && !isTransactionError(err) {
		logger.Errorf("UpdateDirectorySizeIfStale failed for source=%s path=%s: %v", source, path, err)
		return false, err
	}

	// Check if update actually happened (directory might have been modified during scan)
	rowsAffected, _ := result.RowsAffected()
	return rowsAffected > 0, nil
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

// RecalculateDirectorySizes recalculates and updates all directory sizes based on their children.
// This uses a bottom-up approach (deepest directories first) to avoid redundant SUM queries.
func (db *IndexDB) RecalculateDirectorySizes(source, pathPrefix string) (int, error) {
	// 1. Get all directories under the path prefix, ordered by depth (deepest first)
	// Depth is determined by counting slashes in the path
	// Use range query on PRIMARY KEY (source, path) for optimal index utilization
	nextPrefix := getNextPathPrefix(pathPrefix)
	query := `
	SELECT path FROM index_items
	WHERE source = ? AND is_dir = 1 AND path >= ? AND path < ?
	ORDER BY LENGTH(path) - LENGTH(REPLACE(path, '/', '')) DESC
	`

	rows, err := db.Query(query, source, pathPrefix, nextPrefix)
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
		if err = rows.Scan(&path); err != nil {
			return 0, err
		}
		directories = append(directories, path)
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}

	if len(directories) == 0 {
		return 0, nil
	}

	// 2. Start a transaction for bulk updates
	tx, err := db.BeginTransaction()
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}
	// No defer rollback - data integrity not critical, performance is priority

	// Batch calculate all directory sizes in a single query to avoid N+1 problem
	// This is much more efficient than querying each directory individually
	nowUnix := time.Now().Unix()

	// Create placeholders for directory paths
	dirPlaceholders := make([]string, len(directories))
	dirArgs := make([]interface{}, len(directories)+1)
	dirArgs[0] = source
	for i, dirPath := range directories {
		dirPlaceholders[i] = "?"
		dirArgs[i+1] = dirPath
	}

	// Calculate sizes for all directories in one query using GROUP BY
	// We only sum DIRECT children because we are processing bottom-up
	sizeQuery := fmt.Sprintf(`
		SELECT parent_path, COALESCE(SUM(size), 0) as total_size
		FROM index_items
		WHERE source = ? AND parent_path IN (%s)
		GROUP BY parent_path
	`, strings.Join(dirPlaceholders, ","))

	sizeRows, err := tx.Query(sizeQuery, dirArgs...)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}
	defer sizeRows.Close()

	// Map directory paths to their calculated sizes
	sizeMap := make(map[string]int64)
	for sizeRows.Next() {
		var parentPath string
		var totalSize int64
		if err := sizeRows.Scan(&parentPath, &totalSize); err != nil {
			return 0, err
		}
		sizeMap[parentPath] = totalSize
	}
	if err := sizeRows.Err(); err != nil {
		return 0, err
	}

	// Batch update all directories using CASE statements (more efficient than individual updates)
	if len(sizeMap) == 0 {
		return 0, nil
	}

	var caseParts []string
	var updateArgs []interface{}
	var updatePaths []string
	var updatePlaceholders []string

	for dirPath, totalSize := range sizeMap {
		caseParts = append(caseParts, "WHEN path = ? THEN ?")
		updateArgs = append(updateArgs, dirPath, totalSize)
		updatePaths = append(updatePaths, dirPath)
		updatePlaceholders = append(updatePlaceholders, "?")
	}

	updateQuery := fmt.Sprintf(`
		UPDATE index_items 
		SET size = CASE %s END,
		    last_updated = ?
		WHERE source = ? AND path IN (%s)
	`, strings.Join(caseParts, " "), strings.Join(updatePlaceholders, ","))

	updateArgs = append(updateArgs, nowUnix, source)
	for _, path := range updatePaths {
		updateArgs = append(updateArgs, path)
	}

	result, err := tx.Exec(updateQuery, updateArgs...)
	if err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}

	rowsAffected, _ := result.RowsAffected()
	updateCount := int(rowsAffected)

	// 4. Commit transaction
	if err := tx.Commit(); err != nil {
		if isBusyError(err) || isTransactionError(err) {
			return 0, nil
		}
		return 0, err
	}

	return updateCount, nil
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

// escapeLikePattern escapes special LIKE characters (_ and %) in a string
// to make it safe for use in LIKE queries with ESCAPE '\'
// Kept for backward compatibility, but range queries are preferred
func escapeLikePattern(pattern string) string {
	escaped := strings.ReplaceAll(pattern, "\\", "\\\\") // Escape backslashes first
	escaped = strings.ReplaceAll(escaped, "_", "\\_")
	escaped = strings.ReplaceAll(escaped, "%", "\\%")
	return escaped
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
