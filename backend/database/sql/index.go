package sql

import (
	"database/sql"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// IndexDB manages the SQLite database for the file index.
// It wraps the underlying sql.DB connection and provides type-safe methods.
type IndexDB struct {
	*TempDB
}

// NewIndexDB creates a new index database in the cache directory.
// It uses the standard TempDB configuration optimized for performance.
func NewIndexDB(name string) (*IndexDB, error) {
	// Create a persistent-ish temp DB (ID based on source name)
	// We might want to clear it on startup or reuse it?
	// For now, let's assume we want a fresh start or we handle migration later.
	// Using "index_" prefix for clarity.
	db, err := NewTempDB("index_"+name, &TempDBConfig{
		CacheSizeKB:   -200000,   // ~200MB cache (negative value = KB)
		MmapSize:      512000000, // 512MB mmap
		Synchronous:   "NORMAL",  // Better safety than OFF, good performance
		EnableLogging: true,
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
func (db *IndexDB) BulkInsertItems(items []*iteminfo.FileInfo) error {
	tx, err := db.BeginTransaction()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
	INSERT OR REPLACE INTO index_items (path, parent_path, name, size, mod_time, type, is_dir, is_hidden, has_preview)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

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
			return err
		}
	}

	return tx.Commit()
}

// GetItem retrieves a single item by path.
func (db *IndexDB) GetItem(path string) (*iteminfo.FileInfo, error) {
	query := `
	SELECT path, name, size, mod_time, type, is_dir, is_hidden, has_preview
	FROM index_items WHERE path = ?
	`
	row := db.QueryRow(query, path)
	return scanItem(row)
}

// GetDirectoryFiles retrieves all children of a directory.
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
