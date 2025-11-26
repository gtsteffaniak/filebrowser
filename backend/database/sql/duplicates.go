package sql

import (
	"database/sql"
	"fmt"
)

// FileLocation represents a file location in the index with metadata
// needed for duplicate detection operations.
type FileLocation struct {
	DirPath        string
	FileIdx        int
	Name           string
	NormalizedName string
	Extension      string
}

// CreateDuplicatesTable creates the files table and indexes needed for duplicate detection.
// This should be called once after creating a TempDB for duplicate operations.
// Indexes are created before data insertion so they're immediately available for queries.
// For small datasets (typical for duplicate detection), the performance difference is negligible.
func (t *TempDB) CreateDuplicatesTable() error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS files (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		dir_path TEXT NOT NULL,
		file_idx INTEGER NOT NULL,
		size INTEGER NOT NULL,
		name TEXT NOT NULL,
		normalized_name TEXT NOT NULL,
		extension TEXT NOT NULL,
		UNIQUE(dir_path, file_idx)
	);
	CREATE INDEX IF NOT EXISTS idx_size ON files(size);
	CREATE INDEX IF NOT EXISTS idx_size_count ON files(size, normalized_name);
	`

	_, err := t.Exec(createTableSQL)
	return err
}

// InsertFileForDuplicates inserts a file entry into the duplicates table.
// This is called during the first pass through the index to stream files into the database.
func (t *TempDB) InsertFileForDuplicates(dirPath string, fileIdx int, size int64, name, normalizedName, extension string) error {
	_, err := t.Exec(
		"INSERT OR IGNORE INTO files (dir_path, file_idx, size, name, normalized_name, extension) VALUES (?, ?, ?, ?, ?, ?)",
		dirPath, fileIdx, size, name, normalizedName, extension,
	)
	return err
}

// GetSizeGroupsForDuplicates queries for all size groups that have 2+ files.
// Returns sizes in descending order (largest first) as a slice, and a map of size -> count.
// This is used to identify potential duplicate groups before detailed comparison.
// The SQL query efficiently filters and sorts, avoiding the need to create intermediate maps.
func (t *TempDB) GetSizeGroupsForDuplicates(minSize int64) ([]int64, map[int64]int, error) {
	// Query to get sizes with 2+ files, sorted by size DESC
	rows, err := t.Query(`
		SELECT size, COUNT(*) as count
		FROM files
		WHERE size >= ?
		GROUP BY size
		HAVING COUNT(*) >= 2
		ORDER BY size DESC
	`, minSize)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query size groups: %w", err)
	}
	defer rows.Close()

	var sizes []int64
	sizeCounts := make(map[int64]int)
	for rows.Next() {
		var size int64
		var count int
		if err := rows.Scan(&size, &count); err != nil {
			return nil, nil, fmt.Errorf("failed to scan row: %w", err)
		}
		sizes = append(sizes, size)
		sizeCounts[size] = count
	}

	return sizes, sizeCounts, rows.Err()
}

// GetFilesBySizeForDuplicates queries for all files with a specific size.
// Used for processing one size group at a time to minimize memory usage.
func (t *TempDB) GetFilesBySizeForDuplicates(size int64) ([]FileLocation, error) {
	rows, err := t.Query(`
		SELECT dir_path, file_idx, name, normalized_name, extension
		FROM files
		WHERE size = ?
		ORDER BY normalized_name
	`, size)
	if err != nil {
		return nil, fmt.Errorf("failed to query files by size: %w", err)
	}
	defer rows.Close()

	var locations []FileLocation
	for rows.Next() {
		var loc FileLocation
		if err := rows.Scan(&loc.DirPath, &loc.FileIdx, &loc.Name, &loc.NormalizedName, &loc.Extension); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		locations = append(locations, loc)
	}

	return locations, rows.Err()
}

// BulkInsertFilesForDuplicates inserts multiple files in a single transaction.
// This is more efficient than calling InsertFileForDuplicates multiple times.
// The transaction must be started by the caller using BeginTransaction().
func BulkInsertFilesForDuplicates(tx *sql.Tx, dirPath string, fileIdx int, size int64, name, normalizedName, extension string) error {
	_, err := tx.Exec(
		"INSERT OR IGNORE INTO files (dir_path, file_idx, size, name, normalized_name, extension) VALUES (?, ?, ?, ?, ?, ?)",
		dirPath, fileIdx, size, name, normalizedName, extension,
	)
	return err
}
