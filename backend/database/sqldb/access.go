package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/access"
)

// Access rules SQL operations

// GetAccessRulesBySource retrieves all access rules for a source
func (s *SQLStore) GetAccessRulesBySource(source string) (map[string]*access.AccessRule, error) {
	query := `SELECT path, rule_data FROM access_rules WHERE source = ? ORDER BY path`

	rows, err := s.db.Query(query, source)
	if err != nil {
		return nil, fmt.Errorf("failed to get access rules: %w", err)
	}
	defer rows.Close()

	rules := make(map[string]*access.AccessRule)
	for rows.Next() {
		var path string
		var ruleDataJSON []byte

		err := rows.Scan(&path, &ruleDataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan access rule: %w", err)
		}

		var rule access.AccessRule
		if err := json.Unmarshal(ruleDataJSON, &rule); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rule data: %w", err)
		}

		rules[path] = &rule
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating access rules: %w", err)
	}

	return rules, nil
}

// GetAccessRule retrieves a specific access rule
func (s *SQLStore) GetAccessRule(source, path string) (*access.AccessRule, error) {
	query := `SELECT rule_data FROM access_rules WHERE source = ? AND path = ?`

	var ruleDataJSON []byte
	err := s.db.QueryRow(query, source, path).Scan(&ruleDataJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("access rule not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get access rule: %w", err)
	}

	var rule access.AccessRule
	if err := json.Unmarshal(ruleDataJSON, &rule); err != nil {
		return nil, fmt.Errorf("failed to unmarshal rule data: %w", err)
	}

	return &rule, nil
}

// GetAllAccessRules retrieves all access rules organized by source
func (s *SQLStore) GetAllAccessRules() (access.SourceRuleMap, error) {
	query := `SELECT source, path, rule_data FROM access_rules ORDER BY source, path`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all access rules: %w", err)
	}
	defer rows.Close()

	allRules := make(access.SourceRuleMap)
	for rows.Next() {
		var source, path string
		var ruleDataJSON []byte

		err := rows.Scan(&source, &path, &ruleDataJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan access rule: %w", err)
		}

		var rule access.AccessRule
		if err := json.Unmarshal(ruleDataJSON, &rule); err != nil {
			return nil, fmt.Errorf("failed to unmarshal rule data: %w", err)
		}

		if allRules[source] == nil {
			allRules[source] = make(access.RuleMap)
		}
		allRules[source][path] = &rule
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating access rules: %w", err)
	}

	return allRules, nil
}

// SaveAccessRule inserts or updates an access rule
func (s *SQLStore) SaveAccessRule(source, path string, rule *access.AccessRule) error {
	ruleDataJSON, err := json.Marshal(rule)
	if err != nil {
		return fmt.Errorf("failed to marshal rule data: %w", err)
	}

	query := `INSERT OR REPLACE INTO access_rules (source, path, rule_data) VALUES (?, ?, ?)`
	_, err = s.db.Exec(query, source, path, ruleDataJSON)
	if err != nil {
		return fmt.Errorf("failed to save access rule: %w", err)
	}

	return nil
}

// DeleteAccessRule deletes an access rule
func (s *SQLStore) DeleteAccessRule(source, path string) error {
	query := `DELETE FROM access_rules WHERE source = ? AND path = ?`
	result, err := s.db.Exec(query, source, path)
	if err != nil {
		return fmt.Errorf("failed to delete access rule: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("access rule not found")
	}

	return nil
}

// DeleteAccessRulesBySource deletes all access rules for a source
func (s *SQLStore) DeleteAccessRulesBySource(source string) error {
	query := `DELETE FROM access_rules WHERE source = ?`
	_, err := s.db.Exec(query, source)
	if err != nil {
		return fmt.Errorf("failed to delete access rules: %w", err)
	}
	return nil
}
