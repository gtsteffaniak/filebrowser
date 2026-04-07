package sqldb

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// User SQL operations

// UserData holds all the non-queryable user fields in JSON
type UserData struct {
	Password         string                     `json:"password,omitempty"`
	Scopes           []users.SourceScope        `json:"scopes"`
	Tokens           map[string]users.AuthToken `json:"tokens,omitempty"`
	TOTPSecret       string                     `json:"totpSecret,omitempty"`
	TOTPNonce        string                     `json:"totpNonce,omitempty"`
	LoginMethod      users.LoginMethod          `json:"loginMethod"`
	OtpEnabled       bool                       `json:"otpEnabled"`
	Version          int                        `json:"version"`
	ShowFirstLogin   bool                       `json:"showFirstLogin"`
	NonAdminEditable users.NonAdminEditable     `json:"settings"`
	FilePermissions  *users.Permissions         `json:"filePermissions,omitempty"`
}

func applyFilePermissionsFromJSON(user *users.User, data *UserData) {
	if data.FilePermissions == nil {
		return
	}
	fp := data.FilePermissions
	user.Permissions.Modify = fp.Modify
	user.Permissions.Share = fp.Share
	user.Permissions.Delete = fp.Delete
	user.Permissions.Create = fp.Create
	user.Permissions.Download = fp.Download
}

func filePermissionsForJSON(user *users.User) *users.Permissions {
	return &users.Permissions{
		Modify:   user.Permissions.Modify,
		Share:    user.Permissions.Share,
		Delete:   user.Permissions.Delete,
		Create:   user.Permissions.Create,
		Download: user.Permissions.Download,
	}
}

func finishUserLoad(user *users.User, userDataJSON []byte) error {
	var userData UserData
	if err := json.Unmarshal(userDataJSON, &userData); err != nil {
		return fmt.Errorf("failed to unmarshal user data: %w", err)
	}
	user.Password = userData.Password
	user.Scopes = userData.Scopes
	user.Tokens = userData.Tokens
	user.TOTPSecret = userData.TOTPSecret
	user.TOTPNonce = userData.TOTPNonce
	user.LoginMethod = userData.LoginMethod
	user.OtpEnabled = userData.OtpEnabled
	user.Version = userData.Version
	user.ShowFirstLogin = userData.ShowFirstLogin
	user.NonAdminEditable = userData.NonAdminEditable
	applyFilePermissionsFromJSON(user, &userData)
	return nil
}

func scanUint64UserID(s string, dest *uint64) error {
	if s == "" {
		*dest = 0
		return nil
	}
	u, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("parse user_id: %w", err)
	}
	*dest = u
	return nil
}

func userIDDBString(id uint64) string {
	return strconv.FormatUint(id, 10)
}

// GetUserByID retrieves a user by stable numeric id (JWT belongsTo, APIs).
func (s *SQLStore) GetUserByID(id uint64) (*users.User, error) {
	if id == 0 {
		return nil, fmt.Errorf("user not found")
	}
	query := `SELECT username, user_id, perm_api, perm_admin, perm_realtime, user_data 
			  FROM users WHERE user_id = ?`

	var user users.User
	var userDataJSON []byte
	var idStr string

	err := s.db.QueryRow(query, userIDDBString(id)).Scan(
		&user.Username,
		&idStr,
		&user.Permissions.Api,
		&user.Permissions.Admin,
		&user.Permissions.Realtime,
		&userDataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if err := scanUint64UserID(idStr, &user.ID); err != nil {
		return nil, err
	}

	if err := finishUserLoad(&user, userDataJSON); err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserByUsername retrieves a user by username
func (s *SQLStore) GetUserByUsername(username string) (*users.User, error) {
	query := `SELECT username, user_id, perm_api, perm_admin, perm_realtime, user_data 
			  FROM users WHERE username = ?`

	var user users.User
	var userDataJSON []byte
	var idStr string

	err := s.db.QueryRow(query, username).Scan(
		&user.Username,
		&idStr,
		&user.Permissions.Api,
		&user.Permissions.Admin,
		&user.Permissions.Realtime,
		&userDataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("user not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if err := scanUint64UserID(idStr, &user.ID); err != nil {
		return nil, err
	}

	if err := finishUserLoad(&user, userDataJSON); err != nil {
		return nil, err
	}
	return &user, nil
}

// ListUsers retrieves all users
func (s *SQLStore) ListUsers() ([]*users.User, error) {
	query := `SELECT username, user_id, perm_api, perm_admin, perm_realtime, user_data 
			  FROM users ORDER BY username`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}
	defer rows.Close()

	var usersList []*users.User
	for rows.Next() {
		var u users.User
		var userDataJSON []byte
		var idStr string
		if err := rows.Scan(
			&u.Username,
			&idStr,
			&u.Permissions.Api,
			&u.Permissions.Admin,
			&u.Permissions.Realtime,
			&userDataJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		if err := scanUint64UserID(idStr, &u.ID); err != nil {
			return nil, err
		}
		if err := finishUserLoad(&u, userDataJSON); err != nil {
			return nil, err
		}
		usersList = append(usersList, &u)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating users: %w", err)
	}

	return usersList, nil
}

// CreateUser inserts a new user. user.ID must be non-zero (state.CreateUser and migration assign ids).
func (s *SQLStore) CreateUser(user *users.User) error {
	if user.ID == 0 {
		return fmt.Errorf("user id must be set before insert")
	}

	userData := UserData{
		Password:         user.Password,
		Scopes:           user.Scopes,
		Tokens:           user.Tokens,
		TOTPSecret:       user.TOTPSecret,
		TOTPNonce:        user.TOTPNonce,
		LoginMethod:      user.LoginMethod,
		OtpEnabled:       user.OtpEnabled,
		Version:          user.Version,
		ShowFirstLogin:   user.ShowFirstLogin,
		NonAdminEditable: user.NonAdminEditable,
		FilePermissions:  filePermissionsForJSON(user),
	}

	userDataJSON, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	query := `INSERT INTO users (user_id, username, perm_api, perm_admin, perm_realtime, user_data) 
			  VALUES (?, ?, ?, ?, ?, ?)`

	_, err = s.db.Exec(query,
		userIDDBString(user.ID),
		user.Username,
		user.Permissions.Api,
		user.Permissions.Admin,
		user.Permissions.Realtime,
		userDataJSON,
	)
	if err != nil {
		if err.Error() == "UNIQUE constraint failed: users.username" {
			return fmt.Errorf("user with provided username already exists")
		}
		if err.Error() == "UNIQUE constraint failed: users.user_id" {
			return fmt.Errorf("user with provided id already exists")
		}
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

// UpdateUser updates an existing user by username
func (s *SQLStore) UpdateUser(user *users.User) error {
	userData := UserData{
		Password:         user.Password,
		Scopes:           user.Scopes,
		Tokens:           user.Tokens,
		TOTPSecret:       user.TOTPSecret,
		TOTPNonce:        user.TOTPNonce,
		LoginMethod:      user.LoginMethod,
		OtpEnabled:       user.OtpEnabled,
		Version:          user.Version,
		ShowFirstLogin:   user.ShowFirstLogin,
		NonAdminEditable: user.NonAdminEditable,
		FilePermissions:  filePermissionsForJSON(user),
	}

	userDataJSON, err := json.Marshal(userData)
	if err != nil {
		return fmt.Errorf("failed to marshal user data: %w", err)
	}

	query := `UPDATE users SET username = ?, perm_api = ?, perm_admin = ?, 
			  perm_realtime = ?, user_data = ? WHERE user_id = ?`

	result, err := s.db.Exec(query,
		user.Username,
		user.Permissions.Api,
		user.Permissions.Admin,
		user.Permissions.Realtime,
		userDataJSON,
		userIDDBString(user.ID),
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
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

// UpdateUserUsername updates the row for user.ID, including a new username (oldName is unused; kept for callers).
func (s *SQLStore) UpdateUserUsername(oldName string, user *users.User) error {
	_ = oldName
	return s.UpdateUser(user)
}

// DeleteUserByID deletes a user by stable id (non-zero only).
func (s *SQLStore) DeleteUserByID(id uint64) error {
	if id == 0 {
		return fmt.Errorf("user not found")
	}
	query := `DELETE FROM users WHERE user_id = ?`
	result, err := s.db.Exec(query, userIDDBString(id))
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
