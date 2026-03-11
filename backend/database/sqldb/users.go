package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// User SQL operations

// GetUserByID retrieves a user by ID
func (s *SQLStore) GetUserByID(id uint) (*users.User, error) {
	query := `SELECT user_data FROM users WHERE id = ?`

	var userDataJSON []byte
	err := s.db.QueryRow(query, id).Scan(&userDataJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user users.User
	if err := json.Unmarshal(userDataJSON, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *SQLStore) GetUserByUsername(username string) (*users.User, error) {
	query := `SELECT user_data FROM users WHERE username = ?`

	var userDataJSON []byte
	err := s.db.QueryRow(query, username).Scan(&userDataJSON)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	var user users.User
	if err := json.Unmarshal(userDataJSON, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
	}

	return &user, nil
}

// ListUsers retrieves all users
func (s *SQLStore) ListUsers() ([]*users.User, error) {
	query := `SELECT user_data FROM users ORDER BY username`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var usersList []*users.User
	for rows.Next() {
		var userDataJSON []byte
		if err := rows.Scan(&userDataJSON); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}

		var user users.User
		if err := json.Unmarshal(userDataJSON, &user); err != nil {
			return nil, fmt.Errorf("failed to unmarshal user data: %w", err)
		}

		usersList = append(usersList, &user)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return usersList, nil
}

// CreateUser creates a new user in the database
// The database will enforce username uniqueness via UNIQUE constraint
func (s *SQLStore) CreateUser(user *users.User) error {
	// Marshal entire user to JSON
	userDataJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	// Insert new user - database will auto-increment ID
	query := `INSERT INTO users (username, perm_api, perm_admin, perm_modify, 
			  perm_share, perm_realtime, perm_delete, perm_create, perm_download, user_data) 
			  VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := s.db.Exec(query,
		user.Username,
		user.Permissions.Api,
		user.Permissions.Admin,
		user.Permissions.Modify,
		user.Permissions.Share,
		user.Permissions.Realtime,
		user.Permissions.Delete,
		user.Permissions.Create,
		user.Permissions.Download,
		userDataJSON,
	)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.username" {
			return fmt.Errorf("user with provided username already exists")
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	// Get the auto-generated ID and update the user struct
	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}
	user.ID = uint(id)

	return nil
}

// UpdateUser updates an existing user by ID
func (s *SQLStore) UpdateUser(user *users.User) error {
	// Marshal entire user to JSON
	userDataJSON, err := json.Marshal(user)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	// Update existing user
	query := `UPDATE users SET username = ?, perm_api = ?, perm_admin = ?, 
			  perm_modify = ?, perm_share = ?, perm_realtime = ?, perm_delete = ?, 
			  perm_create = ?, perm_download = ?, user_data = ? WHERE id = ?`

	result, err := s.db.Exec(query,
		user.Username,
		user.Permissions.Api,
		user.Permissions.Admin,
		user.Permissions.Modify,
		user.Permissions.Share,
		user.Permissions.Realtime,
		user.Permissions.Delete,
		user.Permissions.Create,
		user.Permissions.Download,
		userDataJSON,
		user.ID,
	)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.username" {
			return fmt.Errorf("user with provided username already exists")
		}
		return fmt.Errorf("failed to update user: %w", err)
	}

	// Check if user exists
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user by ID
func (s *SQLStore) DeleteUser(id uint) error {
	query := `DELETE FROM users WHERE id = ?`
	result, err := s.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUserByUsername deletes a user by username
func (s *SQLStore) DeleteUserByUsername(username string) error {
	query := `DELETE FROM users WHERE username = ?`
	result, err := s.db.Exec(query, username)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}
