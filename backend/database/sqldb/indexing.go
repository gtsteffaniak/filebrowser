package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/dbindex"
)

// Index info SQL operations

// GetIndexInfoByPath retrieves index info by path
func (s *SQLStore) GetIndexInfoByPath(path string) (*dbindex.IndexInfo, error) {
	query := `SELECT path, source, complexity, num_dirs, num_files, scanners 
			  FROM index_info WHERE path = ?`

	var info dbindex.IndexInfo
	var scannersJSON []byte

	err := s.db.QueryRow(query, path).Scan(
		&info.Path,
		&info.Source,
		&info.Complexity,
		&info.NumDirs,
		&info.NumFiles,
		&scannersJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("index info not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get index info: %w", err)
	}

	// Unmarshal scanners
	if scannersJSON != nil {
		if err := json.Unmarshal(scannersJSON, &info.Scanners); err != nil {
			return nil, fmt.Errorf("failed to unmarshal scanners: %w", err)
		}
	}

	return &info, nil
}

// GetIndexInfoBySource retrieves all index info for a source
func (s *SQLStore) GetIndexInfoBySource(source string) ([]*dbindex.IndexInfo, error) {
	query := `SELECT path, source, complexity, num_dirs, num_files, scanners 
			  FROM index_info WHERE source = ? ORDER BY path`

	rows, err := s.db.Query(query, source)
	if err != nil {
		return nil, fmt.Errorf("failed to get index info by source: %w", err)
	}
	defer rows.Close()

	return s.scanIndexInfo(rows)
}

// ListAllIndexInfo retrieves all index info
func (s *SQLStore) ListAllIndexInfo() ([]*dbindex.IndexInfo, error) {
	query := `SELECT path, source, complexity, num_dirs, num_files, scanners 
			  FROM index_info ORDER BY path`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list index info: %w", err)
	}
	defer rows.Close()

	return s.scanIndexInfo(rows)
}

// SaveIndexInfo inserts or updates index info
func (s *SQLStore) SaveIndexInfo(info *dbindex.IndexInfo) error {
	scannersJSON, err := json.Marshal(info.Scanners)
	if err != nil {
		return fmt.Errorf("failed to marshal scanners: %w", err)
	}

	query := `INSERT OR REPLACE INTO index_info 
			  (path, source, complexity, num_dirs, num_files, scanners) 
			  VALUES (?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		info.Path,
		info.Source,
		info.Complexity,
		info.NumDirs,
		info.NumFiles,
		scannersJSON,
	)
	if err != nil {
		return fmt.Errorf("failed to save index info: %w", err)
	}

	return nil
}

// DeleteIndexInfo deletes index info by path
func (s *SQLStore) DeleteIndexInfo(path string) error {
	query := `DELETE FROM index_info WHERE path = ?`
	result, err := s.db.Exec(query, path)
	if err != nil {
		return fmt.Errorf("failed to delete index info: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("index info not found")
	}

	return nil
}

// DeleteIndexInfoBySource deletes all index info for a source
func (s *SQLStore) DeleteIndexInfoBySource(source string) error {
	query := `DELETE FROM index_info WHERE source = ?`
	_, err := s.db.Exec(query, source)
	if err != nil {
		return fmt.Errorf("failed to delete index info by source: %w", err)
	}
	return nil
}

// scanIndexInfo is a helper to scan multiple index info rows
func (s *SQLStore) scanIndexInfo(rows *sql.Rows) ([]*dbindex.IndexInfo, error) {
	var infoList []*dbindex.IndexInfo
	for rows.Next() {
		var info dbindex.IndexInfo
		var scannersJSON []byte

		err := rows.Scan(
			&info.Path,
			&info.Source,
			&info.Complexity,
			&info.NumDirs,
			&info.NumFiles,
			&scannersJSON,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan index info: %w", err)
		}

		// Unmarshal scanners
		if scannersJSON != nil {
			if err := json.Unmarshal(scannersJSON, &info.Scanners); err != nil {
				return nil, fmt.Errorf("failed to unmarshal scanners: %w", err)
			}
		}

		infoList = append(infoList, &info)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating index info: %w", err)
	}

	return infoList, nil
}
