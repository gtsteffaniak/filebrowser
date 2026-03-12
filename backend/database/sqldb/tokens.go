package sqldb

import (
	"fmt"
	"time"
)

// Token SQL operations

// SaveRevokedToken adds a token hash to the revoked tokens table
func (s *SQLStore) SaveRevokedToken(tokenHash string) error {
	query := `INSERT OR IGNORE INTO revoked_tokens (token_hash, revoked_at) VALUES (?, ?)`
	_, err := s.db.Exec(query, tokenHash, time.Now().Unix())
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

// GetAllRevokedTokens retrieves all revoked token hashes
func (s *SQLStore) GetAllRevokedTokens() (map[string]struct{}, error) {
	query := `SELECT token_hash FROM revoked_tokens`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get revoked tokens: %w", err)
	}
	defer rows.Close()

	revokedTokens := make(map[string]struct{})
	for rows.Next() {
		var tokenHash string
		if err := rows.Scan(&tokenHash); err != nil {
			return nil, fmt.Errorf("failed to scan revoked token: %w", err)
		}
		revokedTokens[tokenHash] = struct{}{}
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

// SaveHashedToken saves a token hash to user ID mapping
func (s *SQLStore) SaveHashedToken(tokenHash string, userID uint) error {
	query := `INSERT OR REPLACE INTO hashed_tokens (token_hash, user_id) VALUES (?, ?)`
	_, err := s.db.Exec(query, tokenHash, userID)
	if err != nil {
		return fmt.Errorf("failed to save hashed token: %w", err)
	}
	return nil
}

// GetUserIDByTokenHash retrieves the user ID for a token hash
func (s *SQLStore) GetUserIDByTokenHash(tokenHash string) (uint, error) {
	query := `SELECT user_id FROM hashed_tokens WHERE token_hash = ?`
	var userID uint
	err := s.db.QueryRow(query, tokenHash).Scan(&userID)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return 0, fmt.Errorf("token not found")
		}
		return 0, fmt.Errorf("failed to get user ID by token: %w", err)
	}
	return userID, nil
}

// GetAllHashedTokens retrieves all token hash to user ID mappings
func (s *SQLStore) GetAllHashedTokens() (map[string]uint, error) {
	query := `SELECT token_hash, user_id FROM hashed_tokens`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get hashed tokens: %w", err)
	}
	defer rows.Close()

	hashedTokens := make(map[string]uint)
	for rows.Next() {
		var tokenHash string
		var userID uint
		if err := rows.Scan(&tokenHash, &userID); err != nil {
			return nil, fmt.Errorf("failed to scan hashed token: %w", err)
		}
		hashedTokens[tokenHash] = userID
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

// DeleteHashedTokensByUserID removes all token hashes for a user
func (s *SQLStore) DeleteHashedTokensByUserID(userID uint) error {
	query := `DELETE FROM hashed_tokens WHERE user_id = ?`
	_, err := s.db.Exec(query, userID)
	if err != nil {
		return fmt.Errorf("failed to delete hashed tokens by user: %w", err)
	}
	return nil
}
