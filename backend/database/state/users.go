package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// User operations

// GetUser retrieves a user by ID from the in-memory cache
// Returns a copy of the user to prevent accidental modifications to the cache
func GetUser(id uint) (*users.User, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	user, exists := usersByID[id]
	if !exists {
		return nil, errors.ErrNotExist
	}
	
	// Return a deep copy to prevent modifications to the cached object
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
	
	return &userCopy, nil
}

// GetUserByUsername retrieves a user by username from the in-memory cache
// Returns a copy of the user to prevent accidental modifications to the cache
func GetUserByUsername(username string) (*users.User, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	user, exists := usersByName[username]
	if !exists {
		return nil, errors.ErrNotExist
	}
	
	// Return a deep copy to prevent modifications to the cached object
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
	
	return &userCopy, nil
}

// GetAllUsers returns all users from the in-memory cache
// Returns copies of users to prevent accidental modifications to the cache
func GetAllUsers() ([]*users.User, error) {
	mux.RLock()
	defer mux.RUnlock()
	
	usersList := make([]*users.User, 0, len(usersByID))
	for _, user := range usersByID {
		// Return deep copies to prevent modifications to the cached objects
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
		
		usersList = append(usersList, &userCopy)
	}
	return usersList, nil
}

// SaveUser saves a user (insert or update) with write-through to SQL
func SaveUser(user *users.User, hashPassword bool) error {
	// Hash password if needed
	if hashPassword && user.Password != "" {
		hashedPassword, err := users.HashPwd(user.Password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		user.Password = hashedPassword
	}
	
	mux.Lock()
	defer mux.Unlock()
	
	// Write through to SQL
	err := sqlStore.SaveUser(user)
	if err != nil {
		return err
	}
	
	// Update cache
	usersByID[user.ID] = user
	usersByName[user.Username] = user
	
	return nil
}

// UpdateUser updates a user with write-through to SQL
func UpdateUser(user *users.User) error {
	return SaveUser(user, false)
}

// DeleteUser deletes a user by ID
func DeleteUser(id uint) error {
	mux.Lock()
	defer mux.Unlock()
	
	// Get username before deleting
	user, exists := usersByID[id]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Delete from SQL
	err := sqlStore.DeleteUser(id)
	if err != nil {
		return err
	}
	
	// Remove from cache
	delete(usersByID, id)
	delete(usersByName, user.Username)
	
	return nil
}

// DeleteUserByUsername deletes a user by username
func DeleteUserByUsername(username string) error {
	mux.Lock()
	defer mux.Unlock()
	
	// Get ID before deleting
	user, exists := usersByName[username]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	// Delete from SQL
	err := sqlStore.DeleteUserByUsername(username)
	if err != nil {
		return err
	}
	
	// Remove from cache
	delete(usersByID, user.ID)
	delete(usersByName, username)
	
	return nil
}

// CreateUser creates a new user
func CreateUser(userInfo users.User, permissions users.Permissions) error {
	newUser := &userInfo
	if userInfo.LoginMethod == "password" {
		if userInfo.Password == "" {
			return fmt.Errorf("password is required to create a password login user")
		}
	} else {
		hashpass, err := users.HashPwd(userInfo.Username)
		if err != nil {
			return err
		}
		newUser.Password = hashpass
	}
	if userInfo.Username == "" {
		return fmt.Errorf("username is required to create a user")
	}
	newUser.Permissions = permissions
	return SaveUser(newUser, true)
}

// AddUserToken adds an API token to a user
func AddUserToken(userID uint, token users.AuthToken) error {
	mux.Lock()
	defer mux.Unlock()
	
	user, exists := usersByID[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	if user.Tokens == nil {
		user.Tokens = make(map[string]users.AuthToken)
	}
	user.Tokens[token.Name] = token
	
	// Write through to SQL
	err := sqlStore.SaveUser(user)
	if err != nil {
		return err
	}
	
	return nil
}

// DeleteUserToken removes an API token from a user
func DeleteUserToken(userID uint, tokenName string) error {
	mux.Lock()
	defer mux.Unlock()
	
	user, exists := usersByID[userID]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	if user.Tokens != nil {
		delete(user.Tokens, tokenName)
	}
	
	// Write through to SQL
	err := sqlStore.SaveUser(user)
	if err != nil {
		return err
	}
	
	return nil
}
