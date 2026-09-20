package sqldb

import (
	"fmt"
	"strconv"
)

// HashedTokenRecord is a persisted bearer-token hash → owner mapping.
type HashedTokenRecord struct {
	UserID    uint64
	IsSession bool
}

// SaveRevokedToken persists a revocation. revokedAt is the Unix timestamp of the
// revocation; 0 marks an immediate revocation (no grace window).
func (s *SQLStore) SaveRevokedToken(tokenHash string, revokedAt int64) error {
	query := `INSERT OR REPLACE INTO revoked_tokens (token_hash, revoked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, tokenHash, revokedAt)
	if err != nil {
		return fmt.Errorf("failed to save revoked token: %w", err)
	}
	return nil
}

// IsTokenRevoked checks if a token hash is in the revoked tokens table
func (s *SQLStore) IsTokenRevoked(tokenHash string) (bool, error) {
	query := `SELECT 1 FROM revoked_tokens WHERE token_hash = ?`
	var exists int
	err := s.db.QueryRow(query, tokenHash).Scan(&exists)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return false, nil
		}
		return false, fmt.Errorf("failed to check revoked token: %w", err)
	}
	return true, nil
}

// GetAllRevokedTokens retrieves all revoked token hashes mapped to their
// revocation timestamp (0 = immediate revocation).
func (s *SQLStore) GetAllRevokedTokens() (map[string]int64, error) {
	query := `SELECT token_hash, revoked_at FROM revoked_tokens`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get revoked tokens: %w", err)
	}
	defer rows.Close()

	revokedTokens := make(map[string]int64)
	for rows.Next() {
		var tokenHash string
		var revokedAt int64
		if err := rows.Scan(&tokenHash, &revokedAt); err != nil {
			return nil, fmt.Errorf("failed to scan revoked token: %w", err)
		}
		revokedTokens[tokenHash] = revokedAt
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating revoked tokens: %w", err)
	}

	return revokedTokens, nil
}

// DeleteRevokedToken removes a token hash from the revoked tokens table
func (s *SQLStore) DeleteRevokedToken(tokenHash string) error {
	query := `DELETE FROM revoked_tokens WHERE token_hash = ?`
	_, err := s.db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete revoked token: %w", err)
	}
	return nil
}

// SaveHashedToken saves a token hash to owner user_id mapping (decimal text).
// isSession records that the bearer token is a FileBrowser session token rather
// than a named API token, so auth can reject non-session tokens without caps.
func (s *SQLStore) SaveHashedToken(tokenHash string, userID uint64, isSession bool) error {
	query := `INSERT OR REPLACE INTO hashed_tokens (token_hash, user_id, is_session) VALUES (?, ?, ?)`
	_, err := s.db.Exec(query, tokenHash, strconv.FormatUint(userID, 10), isSession)
	if err != nil {
		return fmt.Errorf("failed to save hashed token: %w", err)
	}
	return nil
}

// GetUserIDByTokenHash returns the owner user id for a token hash.
func (s *SQLStore) GetUserIDByTokenHash(tokenHash string) (uint64, error) {
	query := `SELECT user_id FROM hashed_tokens WHERE token_hash = ?`
	var idStr string
	err := s.db.QueryRow(query, tokenHash).Scan(&idStr)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, fmt.Errorf("token not found")
		}
		return 0, fmt.Errorf("failed to get user id by token: %w", err)
	}
	return strconv.ParseUint(idStr, 10, 64)
}

// GetAllHashedTokens retrieves all token hash → owner mappings.
func (s *SQLStore) GetAllHashedTokens() (map[string]HashedTokenRecord, error) {
	query := `SELECT token_hash, user_id, is_session FROM hashed_tokens`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get hashed tokens: %w", err)
	}
	defer rows.Close()

	hashedTokens := make(map[string]HashedTokenRecord)
	for rows.Next() {
		var tokenHash, idStr string
		var isSession bool
		if err := rows.Scan(&tokenHash, &idStr, &isSession); err != nil {
			return nil, fmt.Errorf("failed to scan hashed token: %w", err)
		}
		uid, err := strconv.ParseUint(idStr, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid user_id in hashed_tokens: %w", err)
		}
		hashedTokens[tokenHash] = HashedTokenRecord{UserID: uid, IsSession: isSession}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating hashed tokens: %w", err)
	}

	return hashedTokens, nil
}

// DeleteHashedToken removes a token hash mapping
func (s *SQLStore) DeleteHashedToken(tokenHash string) error {
	query := `DELETE FROM hashed_tokens WHERE token_hash = ?`
	_, err := s.db.Exec(query, tokenHash)
	if err != nil {
		return fmt.Errorf("failed to delete hashed token: %w", err)
	}
	return nil
}

// DeleteHashedTokensByUserID removes all token hashes for an owner user id.
func (s *SQLStore) DeleteHashedTokensByUserID(userID uint64) error {
	query := `DELETE FROM hashed_tokens WHERE user_id = ?`
	_, err := s.db.Exec(query, strconv.FormatUint(userID, 10))
	if err != nil {
		return fmt.Errorf("failed to delete hashed tokens by user: %w", err)
	}
	return nil
}
