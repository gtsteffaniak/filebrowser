package state

import (
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// usersBackend implements users.StorageBackend using state
type usersBackend struct{}

func (u usersBackend) GetBy(id interface{}) (*users.User, error) {
	switch v := id.(type) {
	case string:
		return GetUserByUsername(v)
	case uint:
		return GetUser(v)
	default:
		return nil, errors.ErrInvalidDataType
	}
}

func (u usersBackend) Gets() ([]*users.User, error) {
	return GetAllUsers()
}

func (u usersBackend) Save(user *users.User, changePass, disableScopeChange bool) error {
	return SaveUser(user, changePass)
}

func (u usersBackend) Update(user *users.User, adminActor bool, fields ...string) error {
	return UpdateUser(user)
}

func (u usersBackend) DeleteByID(id uint) error {
	return DeleteUser(id)
}

func (u usersBackend) DeleteByUsername(username string) error {
	return DeleteUserByUsername(username)
}
