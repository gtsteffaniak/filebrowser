package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// usersBackend implements users.StorageBackend using state
type usersBackend struct{}

func (u usersBackend) GetBy(id uint64) (*users.User, error) {
	if id == 0 {
		return nil, errors.ErrNotExist
	}
	user, err := GetUser(id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (u usersBackend) Gets() ([]*users.User, error) {
	usersList, err := GetAllUsers()
	if err != nil {
		return nil, err
	}
	// Convert values to pointers for backward compatibility
	result := make([]*users.User, len(usersList))
	for i := range usersList {
		result[i] = &usersList[i]
	}
	return result, nil
}

func (u usersBackend) Save(user *users.User, changePass, disableScopeChange bool) error {
	// Check if user exists by trying to get it
	_, err := GetUserByUsername(user.Username)
	if err != nil {
		// User doesn't exist - create new user
		// Extract plaintext password if changePass is true
		plaintextPassword := ""
		if changePass && user.Password != "" {
			plaintextPassword = user.Password
		}
		return CreateUser(user, plaintextPassword)
	}
	
	// User exists - update existing user (full update, no fields specified)
	// Extract plaintext password if changePass is true
	plaintextPassword := ""
	if changePass && user.Password != "" {
		plaintextPassword = user.Password
	}
	return UpdateUser(user, plaintextPassword)
}

func (u usersBackend) Update(user *users.User, adminActor bool, fields ...string) error {
	// Patch update - only update specified fields
	// No password change for Update method
	return UpdateUser(user, "", fields...)
}

func (u usersBackend) DeleteByID(id uint64) error {
	return DeleteUser(id)
}

