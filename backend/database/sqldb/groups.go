package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/access"
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
