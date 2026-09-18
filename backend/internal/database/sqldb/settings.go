package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
)

// Settings SQL operations

// GetSetting retrieves a setting by key
func (s *SQLStore) GetSetting(key string) ([]byte, error) {
	query := `SELECT value FROM settings WHERE key = ?`
	
	var value []byte
	err := s.db.QueryRow(query, key).Scan(&value)
	
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("setting not found: %s", key)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get setting: %w", err)
	}
	
	return value, nil
}

// SaveSetting inserts or updates a setting
func (s *SQLStore) SaveSetting(key string, value interface{}) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
	if err := s.saveSettingQuery(s.db.Exec, query, key, value); err != nil {
		return fmt.Errorf("failed to save setting: %w", err)
	}
	return nil
}

type settingExec func(query string, args ...interface{}) (sql.Result, error)

func (s *SQLStore) saveSettingQuery(exec settingExec, query, key string, value interface{}) error {
	valueJSON, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal setting value: %w", err)
	}
	if _, err := exec(query, key, valueJSON); err != nil {
		return err
	}
	return nil
}

func (s *SQLStore) saveSettingInTx(tx *sql.Tx, key string, value interface{}) error {
	query := `INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)`
	return s.saveSettingQuery(tx.Exec, query, key, value)
}

// SaveSettingTx inserts or updates a setting within an existing transaction.
func (s *SQLStore) SaveSettingTx(tx *sql.Tx, key string, value interface{}) error {
	return s.saveSettingInTx(tx, key, value)
}

// DeleteSetting deletes a setting by key
func (s *SQLStore) DeleteSetting(key string) error {
	query := `DELETE FROM settings WHERE key = ?`
	result, err := s.db.Exec(query, key)
	if err != nil {
		return fmt.Errorf("failed to delete setting: %w", err)
	}
	
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	
	if rowsAffected == 0 {
		return fmt.Errorf("setting not found")
	}
	
	return nil
}

// GetAllSettings retrieves all settings as a map
func (s *SQLStore) GetAllSettings() (map[string][]byte, error) {
	query := `SELECT key, value FROM settings`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all settings: %w", err)
	}
	defer rows.Close()
	
	settings := make(map[string][]byte)
	for rows.Next() {
		var key string
		var value []byte
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings[key] = value
	}
	
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating settings: %w", err)
	}
	
	return settings, nil
}
