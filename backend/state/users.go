package state

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// User operations

// GetUser retrieves a user by stable numeric id from the in-memory cache (JWT belongsTo, admin APIs).
// Returns a value (not pointer) to prevent modifications to the cache
func GetUser(id uint64) (users.User, error) {
	if id == 0 {
		return users.User{}, errors.ErrNotExist
	}
	usersMux.RLock()
	defer usersMux.RUnlock()

	user, exists := usersByID[id]
	if !exists {
		return users.User{}, errors.ErrNotExist
	}

	// Return a value copy - automatically immutable
	userCopy := *user

	// Deep copy slices and maps
	if user.Scopes != nil {
		userCopy.Scopes = make([]users.SourceScope, len(user.Scopes))
		copy(userCopy.Scopes, user.Scopes)
	}

	if user.Tokens != nil {
		userCopy.Tokens = make(map[string]users.AuthToken, len(user.Tokens))
		for k, v := range user.Tokens {
			userCopy.Tokens[k] = v
		}
	}

	if user.SidebarLinks != nil {
		userCopy.SidebarLinks = make([]users.SidebarLink, len(user.SidebarLinks))
		copy(userCopy.SidebarLinks, user.SidebarLinks)
	}

	return userCopy, nil
}

// GetAllUsers returns all users from the in-memory cache
// Returns a value (not pointer) to prevent modifications to the cache
func GetUserByUsername(username string) (users.User, error) {
	usersMux.RLock()
	defer usersMux.RUnlock()

	user, exists := usersByName[username]
	if !exists {
		return users.User{}, errors.ErrNotExist
	}

	// Return a value copy - automatically immutable
	userCopy := *user

	// Deep copy slices and maps
	if user.Scopes != nil {
		userCopy.Scopes = make([]users.SourceScope, len(user.Scopes))
		copy(userCopy.Scopes, user.Scopes)
	}

	if user.Tokens != nil {
		userCopy.Tokens = make(map[string]users.AuthToken, len(user.Tokens))
		for k, v := range user.Tokens {
			userCopy.Tokens[k] = v
		}
	}

	if user.SidebarLinks != nil {
		userCopy.SidebarLinks = make([]users.SidebarLink, len(user.SidebarLinks))
		copy(userCopy.SidebarLinks, user.SidebarLinks)
	}

	return userCopy, nil
}

// UserIDForUsername returns the stable id for a login name (for id-keyed user storage).
func UserIDForUsername(username string) (uint64, error) {
	usersMux.RLock()
	defer usersMux.RUnlock()
	u, ok := usersByName[username]
	if !ok {
		return 0, errors.ErrNotExist
	}
	return u.ID, nil
}

// GetAllUsers returns all users from the in-memory cache
// Returns values (not pointers) to prevent modifications to the cache
func GetAllUsers() ([]users.User, error) {
	usersMux.RLock()
	defer usersMux.RUnlock()

	usersList := make([]users.User, 0, len(usersByName))
	for _, user := range usersByName {
		// Return value copies - automatically immutable
		userCopy := *user

		// Deep copy slices and maps
		if user.Scopes != nil {
			userCopy.Scopes = make([]users.SourceScope, len(user.Scopes))
			copy(userCopy.Scopes, user.Scopes)
		}

		if user.Tokens != nil {
			userCopy.Tokens = make(map[string]users.AuthToken, len(user.Tokens))
			for k, v := range user.Tokens {
				userCopy.Tokens[k] = v
			}
		}

		if user.SidebarLinks != nil {
			userCopy.SidebarLinks = make([]users.SidebarLink, len(user.SidebarLinks))
			copy(userCopy.SidebarLinks, user.SidebarLinks)
		}

		usersList = append(usersList, userCopy)
	}
	return usersList, nil
}

// UserFromAPIToken resolves the user for a validated API JWT: numeric belongsTo id, or minimal tokens
// (hash → user id). Usernames are not used so a reused login name never inherits a previous account's API access.
func UserFromAPIToken(tk users.AuthToken, rawToken string) (users.User, error) {
	if tk.BelongsTo != 0 {
		return GetUser(tk.BelongsTo)
	}
	if uid, ok := accessStorage.GetUserIDFromToken(rawToken); ok {
		return GetUser(uid)
	}
	return users.User{}, errors.ErrNotExist
}

// UserForShareOwner resolves the user who owns a share link.
func UserForShareOwner(link *share.Link) (users.User, error) {
	if link.UserID == 0 {
		return users.User{}, errors.ErrNotExist
	}
	return GetUser(link.UserID)
}

// CreateUser creates a new user with plaintext password
func CreateUser(user *users.User, plaintextPassword string) error {
	if _, exists := usersByName[user.Username]; exists {
		return fmt.Errorf("user with username %s already exists", user.Username)
	}
	// Hash password if provided
	if plaintextPassword != "" {
		hashedPassword, err := utils.HashPwd(plaintextPassword)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
	}

	// Convert scope names to backend paths and create user directories if needed
	adjustedScopes, err := user.GetBackendScopes()
	if err != nil {
		return err
	}
	user.Scopes = adjustedScopes

	// Create user directories and adjust scope paths if createUserDir is enabled
	err = files.MakeUserDirs(user, false)
	if err != nil {
		logger.Error(err.Error())
	}

	usersMux.Lock()
	defer usersMux.Unlock()

	if user.ID == 0 {
		nid, genErr := users.NextRandomUserID()
		if genErr != nil {
			return fmt.Errorf("allocate user id: %w", genErr)
		}
		user.ID = nid
	}

	err = sqlStore.CreateUser(user)
	if err != nil {
		return err
	}

	usersByName[user.Username] = user
	usersByID[user.ID] = user

	return nil
}

// UpdateUser updates an existing user with write-through to SQL
// If plaintextPassword is provided (non-empty), it will be hashed before saving
// If fields are specified, only those fields are updated (patch operation)
// Note: fields should be JSON tag names (e.g., "showFirstLogin") which will be converted to struct field names
func UpdateUser(user *users.User, plaintextPassword string, fields ...string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	// 1. Check if user exists in cache (state)
	var existingUser *users.User
	var exists bool
	if user.ID != 0 {
		existingUser, exists = usersByID[user.ID]
	}
	if !exists {
		existingUser, exists = usersByName[user.Username]
	}
	if !exists || existingUser == nil {
		return fmt.Errorf("user %s not found in cache", user.Username)
	}
	oldUsername := existingUser.Username
	oldUserID := existingUser.ID

	// If no fields specified, update all fields (full replacement)
	updateAll := len(fields) == 0

	if !updateAll {
		// 2. Patch operation - selectively copy specified fields using reflection
		existingVal := reflect.ValueOf(existingUser).Elem()
		newVal := reflect.ValueOf(user).Elem()

		for _, jsonFieldName := range fields {
			// Handle password specially
			if jsonFieldName == "password" || jsonFieldName == "Password" {
				if plaintextPassword != "" {
					hashedPassword, err := utils.HashPwd(plaintextPassword)
					if err != nil {
						return fmt.Errorf("failed to hash password: %w", err)
					}
					existingUser.Password = hashedPassword
				}
				continue
			}

			// Find struct field by JSON tag name (handles embedded structs)
			structFieldName := findFieldByJSONTag(reflect.TypeOf(user).Elem(), jsonFieldName)

			// If not found by JSON tag, try direct field name match (for backwards compatibility)
			if structFieldName == "" {
				structFieldName = jsonFieldName
			}

			// Use reflection to copy the field (FieldByName works with embedded structs)
			existingField := existingVal.FieldByName(structFieldName)
			newField := newVal.FieldByName(structFieldName)

			if existingField.IsValid() && existingField.CanSet() && newField.IsValid() {
				existingField.Set(newField)
			}
		}
	} else {
		// Full update - replace all fields
		if plaintextPassword != "" {
			hashedPassword, err := utils.HashPwd(plaintextPassword)
			if err != nil {
				return fmt.Errorf("failed to hash password: %w", err)
			}
			user.Password = hashedPassword
		} else {
			// Preserve existing password
			user.Password = existingUser.Password
		}

		// Replace entire user pointer
		existingUser = user
	}

	// 3. Write to database
	var err error
	if oldUsername != existingUser.Username {
		if err = sqlStore.UpdateUserUsername(oldUsername, existingUser); err != nil {
			return err
		}
		delete(usersByName, oldUsername)
	} else {
		if err = sqlStore.UpdateUser(existingUser); err != nil {
			return err
		}
	}

	// 4. Update cache to match database
	if oldUserID != 0 && oldUserID != existingUser.ID {
		delete(usersByID, oldUserID)
	}
	if existingUser.ID != 0 {
		usersByID[existingUser.ID] = existingUser
	}
	usersByName[existingUser.Username] = existingUser

	return nil
}

// findFieldByJSONTag recursively searches for a struct field by its JSON tag name
// Handles embedded structs by searching through all levels
func findFieldByJSONTag(t reflect.Type, jsonTag string) string {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Check if this field's JSON tag matches
		jsonTagValue := field.Tag.Get("json")
		if jsonTagValue != "" {
			// Parse tag (might be "fieldName,omitempty")
			tagName := strings.Split(jsonTagValue, ",")[0]
			if tagName == jsonTag {
				return field.Name
			}
		}

		// If this is an embedded struct (Anonymous field), search recursively
		if field.Anonymous && field.Type.Kind() == reflect.Struct {
			if nestedFieldName := findFieldByJSONTag(field.Type, jsonTag); nestedFieldName != "" {
				// For embedded structs, we can access the field directly by name
				// because Go promotes embedded struct fields
				return nestedFieldName
			}
		}
	}

	return ""
}

// DeleteUser deletes a user by stable numeric id (non-zero).
func DeleteUser(id uint64) error {
	if id == 0 {
		return fmt.Errorf("user not found in state")
	}
	usersMux.Lock()
	defer usersMux.Unlock()

	user, exists := usersByID[id]
	if !exists {
		return fmt.Errorf("user not found in state")
	}

	err := sqlStore.DeleteUserByID(id)
	if err != nil {
		return err
	}

	delete(usersByID, id)
	delete(usersByName, user.Username)

	if accessStorage != nil {
		_ = accessStorage.RemoveHashedTokensForUser(id)
	}

	return nil
}

// DeleteUserByUsername deletes a user by username
func DeleteUserByUsername(username string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	// 1. Check if user exists in state
	user, exists := usersByName[username]
	if !exists {
		return fmt.Errorf("user not found in state")
	}

	err := sqlStore.DeleteUserByUsername(username)
	if err != nil {
		return err
	}

	uid := user.ID
	if user.ID != 0 {
		delete(usersByID, user.ID)
	}
	delete(usersByName, username)

	if accessStorage != nil && uid != 0 {
		_ = accessStorage.RemoveHashedTokensForUser(uid)
	}

	return nil
}

// AddUserToken adds an API token to a user
func AddUserToken(ownerUsername string, token users.AuthToken) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	user, exists := usersByName[ownerUsername]
	if !exists {
		return fmt.Errorf("user not found in state")
	}

	// Check if token already exists
	if user.Tokens != nil {
		if _, tokenExists := user.Tokens[token.Name]; tokenExists {
			return fmt.Errorf("token with name %s already exists for user", token.Name)
		}
	}

	// Prepare the update
	if user.Tokens == nil {
		user.Tokens = make(map[string]users.AuthToken)
	}
	user.Tokens[token.Name] = token

	// 2. Write to database
	err := sqlStore.UpdateUser(user)
	if err != nil {
		return err
	}

	// 3. Cache is already updated since we modified the pointer directly

	return nil
}

// DeleteUserToken removes an API token from a user
func DeleteUserToken(ownerUsername string, tokenName string) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	user, exists := usersByName[ownerUsername]
	if !exists {
		return fmt.Errorf("user not found in state")
	}

	// Check if token exists
	if user.Tokens == nil {
		return fmt.Errorf("user has no tokens")
	}
	if _, tokenExists := user.Tokens[tokenName]; !tokenExists {
		return fmt.Errorf("token with name %s not found for user", tokenName)
	}

	// Prepare the update
	delete(user.Tokens, tokenName)

	// 2. Write to database
	err := sqlStore.UpdateUser(user)
	if err != nil {
		return err
	}

	// 3. Cache is already updated since we modified the pointer directly

	return nil
}
