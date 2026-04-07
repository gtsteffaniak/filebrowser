package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/database/share"
)

func shareUserIDDB(id uint64) string {
	return strconv.FormatUint(id, 10)
}

func scanShareUserID(s string, dest *uint64) error {
	if s == "" {
		*dest = 0
		return nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parse share user_id: %w", err)
	}
	*dest = u
	return nil
}

// GetShareByHash retrieves a share by hash
func (s *SQLStore) GetShareByHash(hash string) (*share.Link, error) {
	query := `SELECT hash, user_id, source, path, expire, downloads, 
			  password_hash, token, user_downloads, share_settings, version 
			  FROM shares WHERE hash = ?`

	var link share.Link
	var userIDStr string
	var userDownloadsJSON, shareSettingsJSON []byte

	err := s.db.QueryRow(query, hash).Scan(
		&link.Hash,
		&userIDStr,
		&link.Source,
		&link.Path,
		&link.Expire,
		&link.Downloads,
		&link.PasswordHash,
		&link.Token,
		&userDownloadsJSON,
		&shareSettingsJSON,
		&link.Version,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("share not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get share: %w", err)
	}
	if err := scanShareUserID(userIDStr, &link.UserID); err != nil {
		return nil, err
	}

	if userDownloadsJSON != nil {
		if err := json.Unmarshal(userDownloadsJSON, &link.UserDownloads); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user downloads: %w", err)
		}
	}
	if err := json.Unmarshal(shareSettingsJSON, &link.CommonShare); err != nil {
		return nil, fmt.Errorf("failed to unmarshal share settings: %w", err)
	}

	return &link, nil
}

// GetSharesByUserID retrieves all non-expired shares for an owner user id.
func (s *SQLStore) GetSharesByUserID(userID uint64) ([]*share.Link, error) {
	now := time.Now().Unix()
	query := `SELECT hash, user_id, source, path, expire, downloads, 
			  password_hash, token, user_downloads, share_settings, version 
			  FROM shares WHERE user_id = ? AND (expire = 0 OR expire > ?) 
			  ORDER BY path`

	rows, err := s.db.Query(query, shareUserIDDB(userID), now)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares by user id: %w", err)
	}
	defer rows.Close()

	return s.scanShares(rows)
}

// GetSharesBySourcePath retrieves shares for a specific source and path
func (s *SQLStore) GetSharesBySourcePath(source, path string) ([]*share.Link, error) {
	query := `SELECT hash, user_id, source, path, expire, downloads, 
			  password_hash, token, user_downloads, share_settings, version 
			  FROM shares WHERE source = ? AND path = ? ORDER BY hash`

	rows, err := s.db.Query(query, source, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares by source/path: %w", err)
	}
	defer rows.Close()

	return s.scanShares(rows)
}

// GetSharesBySourcePathUser retrieves shares for a specific source, path, and owner user id.
func (s *SQLStore) GetSharesBySourcePathUser(source, path string, userID uint64) ([]*share.Link, error) {
	now := time.Now().Unix()
	query := `SELECT hash, user_id, source, path, expire, downloads, 
			  password_hash, token, user_downloads, share_settings, version 
			  FROM shares WHERE source = ? AND path = ? AND user_id = ? 
			  AND (expire = 0 OR expire > ?) ORDER BY hash`

	rows, err := s.db.Query(query, source, path, shareUserIDDB(userID), now)
	if err != nil {
		return nil, fmt.Errorf("failed to get shares: %w", err)
	}
	defer rows.Close()

	return s.scanShares(rows)
}

// GetPermanentShare retrieves a permanent share (expire = 0) for source, path, and owner.
func (s *SQLStore) GetPermanentShare(source, path string, userID uint64) (*share.Link, error) {
	query := `SELECT hash, user_id, source, path, expire, downloads,
			  password_hash, token, user_downloads, share_settings, version
			  FROM shares WHERE source = ? AND path = ? AND user_id = ? AND expire = 0
			  LIMIT 1`

	var link share.Link
	var userIDStr string
	var userDownloadsJSON, shareSettingsJSON []byte

	err := s.db.QueryRow(query, source, path, shareUserIDDB(userID)).Scan(
		&link.Hash,
		&userIDStr,
		&link.Source,
		&link.Path,
		&link.Expire,
		&link.Downloads,
		&link.PasswordHash,
		&link.Token,
		&userDownloadsJSON,
		&shareSettingsJSON,
		&link.Version,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("permanent share not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get permanent share: %w", err)
	}
	if err := scanShareUserID(userIDStr, &link.UserID); err != nil {
		return nil, err
	}

	if userDownloadsJSON != nil {
		if err := json.Unmarshal(userDownloadsJSON, &link.UserDownloads); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user downloads: %w", err)
		}
	}
	if err := json.Unmarshal(shareSettingsJSON, &link.CommonShare); err != nil {
		return nil, fmt.Errorf("failed to unmarshal share settings: %w", err)
	}

	return &link, nil
}

// ListAllShares retrieves all non-expired shares
func (s *SQLStore) ListAllShares() ([]*share.Link, error) {
	now := time.Now().Unix()
	query := `SELECT hash, user_id, source, path, expire, downloads, 
			  password_hash, token, user_downloads, share_settings, version 
			  FROM shares WHERE expire = 0 OR expire > ? ORDER BY path`

	rows, err := s.db.Query(query, now)
	if err != nil {
		return nil, fmt.Errorf("failed to list shares: %w", err)
	}
	defer rows.Close()

	return s.scanShares(rows)
}

// SaveShare inserts or updates a share
func (s *SQLStore) SaveShare(link *share.Link) error {
	userDownloadsJSON, err := json.Marshal(link.UserDownloads)
	if err != nil {
		return fmt.Errorf("failed to marshal user downloads: %w", err)
	}
	shareSettingsJSON, err := json.Marshal(link.CommonShare)
	if err != nil {
		return fmt.Errorf("failed to marshal share settings: %w", err)
	}

	query := `INSERT OR REPLACE INTO shares 
			  (hash, user_id, source, path, expire, downloads, password_hash, 
			   token, user_downloads, share_settings, version) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		link.Hash,
		shareUserIDDB(link.UserID),
		link.Source,
		link.Path,
		link.Expire,
		link.Downloads,
		link.PasswordHash,
		link.Token,
		userDownloadsJSON,
		shareSettingsJSON,
		link.Version,
	)
	if err != nil {
		return fmt.Errorf("failed to save share: %w", err)
	}

	return nil
}

// DeleteShare deletes a share by hash
func (s *SQLStore) DeleteShare(hash string) error {
	query := `DELETE FROM shares WHERE hash = ?`
	result, err := s.db.Exec(query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete share: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("share not found")
	}

	return nil
}

// DeleteExpiredShares deletes all expired shares
func (s *SQLStore) DeleteExpiredShares() (int64, error) {
	now := time.Now().Unix()
	query := `DELETE FROM shares WHERE expire > 0 AND expire <= ?`
	result, err := s.db.Exec(query, now)
	if err != nil {
		return 0, fmt.Errorf("failed to delete expired shares: %w", err)
	}

	return result.RowsAffected()
}

// UpdateSharePath updates the path for a specific share
func (s *SQLStore) UpdateSharePath(hash, newPath string) error {
	query := `UPDATE shares SET path = ? WHERE hash = ?`
	_, err := s.db.Exec(query, newPath, hash)
	if err != nil {
		return fmt.Errorf("failed to update share path: %w", err)
	}
	return nil
}

// UpdateSharesPaths updates paths for shares when a resource is moved
func (s *SQLStore) UpdateSharesPaths(oldSource, oldPath, newSource, newPath string) error {
	query := `UPDATE shares SET source = ?, path = ? WHERE source = ? AND path = ?`
	_, err := s.db.Exec(query, newSource, newPath, oldSource, oldPath)
	if err != nil {
		return fmt.Errorf("failed to update shares paths: %w", err)
	}
	return nil
}

func (s *SQLStore) scanShares(rows *sql.Rows) ([]*share.Link, error) {
	var shares []*share.Link
	for rows.Next() {
		var link share.Link
		var userIDStr string
		var userDownloadsJSON, shareSettingsJSON []byte

		err := rows.Scan(
			&link.Hash,
			&userIDStr,
			&link.Source,
			&link.Path,
			&link.Expire,
			&link.Downloads,
			&link.PasswordHash,
			&link.Token,
			&userDownloadsJSON,
			&shareSettingsJSON,
			&link.Version,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan share: %w", err)
		}
		if err := scanShareUserID(userIDStr, &link.UserID); err != nil {
			return nil, err
		}

		if userDownloadsJSON != nil {
			if err := json.Unmarshal(userDownloadsJSON, &link.UserDownloads); err != nil {
				return nil, fmt.Errorf("failed to unmarshal user downloads: %w", err)
			}
		}
		if err := json.Unmarshal(shareSettingsJSON, &link.CommonShare); err != nil {
			return nil, fmt.Errorf("failed to unmarshal share settings: %w", err)
		}

		shares = append(shares, &link)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating shares: %w", err)
	}

	return shares, nil
}
