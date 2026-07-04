package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Auth methods SQL operations

// GetAuthMethod retrieves an auth method by type
func (s *SQLStore) GetAuthMethod(authType string) ([]byte, error) {
	query := `SELECT config FROM auth_methods WHERE type = ?`
	
	var config []byte
	err := s.db.QueryRow(query, authType).Scan(&config)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("auth method not found: %s", authType)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get auth method: %w", err)
	}
	
	return config, nil
}

// SaveAuthMethod inserts or updates an auth method
func (s *SQLStore) SaveAuthMethod(authType string, config interface{}) error {
	configJSON, err := json.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal auth method config: %w", err)
	}
	
	query := `INSERT OR REPLACE INTO auth_methods (type, config) VALUES (?, ?)`
	_, err = s.db.Exec(query, authType, configJSON)
	if err != nil {
		return fmt.Errorf("failed to save auth method: %w", err)
	}
	
	return nil
}

// DeleteAuthMethod deletes an auth method by type
func (s *SQLStore) DeleteAuthMethod(authType string) error {
	query := `DELETE FROM auth_methods WHERE type = ?`
	result, err := s.db.Exec(query, authType)
	if err != nil {
		return fmt.Errorf("failed to delete auth method: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("auth method not found")
	}
	
	return nil
}

// GetAllAuthMethods retrieves all auth methods as a map
func (s *SQLStore) GetAllAuthMethods() (map[string][]byte, error) {
	query := `SELECT type, config FROM auth_methods`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all auth methods: %w", err)
	}
	defer rows.Close()
	
	authMethods := make(map[string][]byte)
	for rows.Next() {
		var authType string
		var config []byte
		if err := rows.Scan(&authType, &config); err != nil {
			return nil, fmt.Errorf("failed to scan auth method: %w", err)
		}
		authMethods[authType] = config
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating auth methods: %w", err)
	}
	
	return authMethods, nil
}
