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

// SaveHashedToken saves a token hash to owner username mapping
func (s *SQLStore) SaveHashedToken(tokenHash string, username string) error {
	query := `INSERT OR REPLACE INTO hashed_tokens (token_hash, username) VALUES (?, ?)`
	_, err := s.db.Exec(query, tokenHash, username)
	if err != nil {
		return fmt.Errorf("failed to save hashed token: %w", err)
	}
	return nil
}

// GetUsernameByTokenHash retrieves the username for a token hash
func (s *SQLStore) GetUsernameByTokenHash(tokenHash string) (string, error) {
	query := `SELECT username FROM hashed_tokens WHERE token_hash = ?`
	var username string
	err := s.db.QueryRow(query, tokenHash).Scan(&username)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return "", fmt.Errorf("token not found")
		}
		return "", fmt.Errorf("failed to get username by token: %w", err)
	}
	return username, nil
}

// GetAllHashedTokens retrieves all token hash to username mappings
func (s *SQLStore) GetAllHashedTokens() (map[string]string, error) {
	query := `SELECT token_hash, username FROM hashed_tokens`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get hashed tokens: %w", err)
	}
	defer rows.Close()

	hashedTokens := make(map[string]string)
	for rows.Next() {
		var tokenHash, username string
		if err := rows.Scan(&tokenHash, &username); err != nil {
			return nil, fmt.Errorf("failed to scan hashed token: %w", err)
		}
		hashedTokens[tokenHash] = username
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

// DeleteHashedTokensByUsername removes all token hashes for a user
func (s *SQLStore) DeleteHashedTokensByUsername(username string) error {
	query := `DELETE FROM hashed_tokens WHERE username = ?`
	_, err := s.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete hashed tokens by user: %w", err)
	}
	return nil
}
