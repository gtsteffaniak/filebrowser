package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
)

// Groups SQL operations

// GetGroup retrieves a group by name
func (s *SQLStore) GetGroup(name string) (access.StringSet, error) {
	query := `SELECT members FROM groups WHERE name = ?`

	var membersJSON []byte
	err := s.db.QueryRow(query, name).Scan(&membersJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("group not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	var members []string
	if err := json.Unmarshal(membersJSON, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members: %w", err)
	}

	// Convert slice to StringSet
	memberSet := make(access.StringSet)
	for _, member := range members {
		memberSet[member] = struct{}{}
	}

	return memberSet, nil
}

// GetAllGroups retrieves all groups
func (s *SQLStore) GetAllGroups() (access.GroupMap, error) {
	query := `SELECT name, members FROM groups ORDER BY name`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all groups: %w", err)
	}
	defer rows.Close()

	groups := make(access.GroupMap)
	for rows.Next() {
		var name string
		var membersJSON []byte

		err := rows.Scan(&name, &membersJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to scan group: %w", err)
		}

		var members []string
		if err := json.Unmarshal(membersJSON, &members); err != nil {
			return nil, fmt.Errorf("failed to unmarshal members: %w", err)
		}

		// Convert slice to StringSet
		memberSet := make(access.StringSet)
		for _, member := range members {
			memberSet[member] = struct{}{}
		}

		groups[name] = memberSet
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating groups: %w", err)
	}

	return groups, nil
}

// SaveGroup inserts or updates a group
func (s *SQLStore) SaveGroup(name string, members access.StringSet) error {
	// Convert StringSet to slice for JSON
	memberSlice := make([]string, 0, len(members))
	for member := range members {
		memberSlice = append(memberSlice, member)
	}

	membersJSON, err := json.Marshal(memberSlice)
	if err != nil {
		return fmt.Errorf("failed to marshal members: %w", err)
	}

	query := `INSERT OR REPLACE INTO groups (name, members) VALUES (?, ?)`
	_, err = s.db.Exec(query, name, membersJSON)
	if err != nil {
		return fmt.Errorf("failed to save group: %w", err)
	}

	return nil
}

// DeleteGroupWithRules deletes a group and applies access rule row changes in a single
// transaction. If any statement fails nothing is written. A group row that is already
// absent is not an error, so the call is safe to retry.
func (s *SQLStore) DeleteGroupWithRules(name string, save []access.RuleUpsert, remove []access.RuleKey) error {
	tx, err := s.BeginTx()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	rollback := func(cause error) error {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", cause, rbErr)
		}
		return cause
	}
	for _, u := range save {
		ruleDataJSON, err := json.Marshal(u.Rule)
		if err != nil {
			return rollback(fmt.Errorf("failed to marshal rule data: %w", err))
		}
		if _, err := tx.Exec(`INSERT OR REPLACE INTO access_rules (source, path, rule_data) VALUES (?, ?, ?)`, u.Source, u.Path, ruleDataJSON); err != nil {
			return rollback(fmt.Errorf("failed to save access rule: %w", err))
		}
	}
	for _, k := range remove {
		if _, err := tx.Exec(`DELETE FROM access_rules WHERE source = ? AND path = ?`, k.Source, k.Path); err != nil {
			return rollback(fmt.Errorf("failed to delete access rule: %w", err))
		}
	}
	if _, err := tx.Exec(`DELETE FROM groups WHERE name = ?`, name); err != nil {
		return rollback(fmt.Errorf("failed to delete group: %w", err))
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit group delete: %w", err)
	}
	return nil
}

// DeleteGroup deletes a group by name
func (s *SQLStore) DeleteGroup(name string) error {
	query := `DELETE FROM groups WHERE name = ?`
	result, err := s.db.Exec(query, name)
	if err != nil {
		return fmt.Errorf("failed to delete group: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("group not found")
	}

	return nil
}
