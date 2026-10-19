package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
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

// MigrateAccessRulesInsensitive rewrites rule paths for case-insensitive mode and
// persists the source case mode in one transaction.
func (s *SQLStore) MigrateAccessRulesInsensitive(source string, oldPaths []string, newRules access.RuleMap, caseInsensitive bool) error {
	tx, err := s.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin access rule migration: %w", err)
	}
	rollback := func(cause error) error {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", cause, rbErr)
		}
		return cause
	}

	for path, rule := range newRules {
		ruleDataJSON, err := json.Marshal(rule)
		if err != nil {
			return rollback(fmt.Errorf("failed to marshal rule data: %w", err))
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO access_rules (source, path, rule_data) VALUES (?, ?, ?)`, source, path, ruleDataJSON); err != nil {
			return rollback(fmt.Errorf("failed to save access rule: %w", err))
		}
	}
	for _, old := range oldPaths {
		if _, kept := newRules[old]; kept {
			continue
		}
		if _, err := tx.Exec(`DELETE FROM access_rules WHERE source = ? AND path = ?`, source, old); err != nil {
			return rollback(fmt.Errorf("failed to delete access rule: %w", err))
		}
	}
	mode := 0
	if caseInsensitive {
		mode = 1
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO access_source_mode (source, case_insensitive) VALUES (?, ?)`, source, mode); err != nil {
		return rollback(fmt.Errorf("failed to save access source mode: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit access rule migration: %w", err)
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

	if _, err := result.RowsAffected(); err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
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
