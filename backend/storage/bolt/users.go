package bolt

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
	"github.com/gtsteffaniak/filebrowser/backend/files"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
	"github.com/gtsteffaniak/filebrowser/backend/utils"
)

type usersBackend struct {
	db *storm.DB
}

func (st usersBackend) GetBy(i interface{}) (user *users.User, err error) {
	user = &users.User{}

	var arg string
	var val interface{}
	switch i := i.(type) {
	case uint:
		val = i
		arg = "ID"
	case int:
		val = uint(i)
	case string:
		arg = "Username"
		val = i
	default:
		return nil, errors.ErrInvalidDataType
	}

	err = st.db.One(arg, val, user)

	if err != nil {
		if err == storm.ErrNotFound {
			return nil, errors.ErrNotExist
		}
		return nil, err
	}

	return
}

func (st usersBackend) Gets() ([]*users.User, error) {
	var allUsers []*users.User
	err := st.db.All(&allUsers)
	if err == storm.ErrNotFound {
		return nil, errors.ErrNotExist
	}
	if err != nil {
		return allUsers, err
	}
	return allUsers, err
}

func (st usersBackend) Update(user *users.User, actorIsAdmin bool, fields ...string) error {
	existingUser, err := st.GetBy(user.ID)
	if err != nil {
		return err
	}

	fields, err = parseFields(user, fields)
	if err != nil {
		return err
	}

	if !slices.Contains(fields, "Password") {
		user.Password = existingUser.Password
	}

	if !actorIsAdmin {
		err := checkRestrictedFields(existingUser, fields)
		if err != nil {
			return err
		}
	}

	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// converting scopes to map of paths intead of names (names can change)
	if slices.Contains(fields, "scopes") || slices.Contains(fields, "Scopes") {
		adjustedScopes, err := settings.ConvertToBackendScopes(user.Scopes)
		if err != nil {
			return err
		}
		user.Scopes = adjustedScopes
	}
	// Use reflection to access struct fields
	userFields := reflect.ValueOf(user).Elem() // Get struct value

	for _, field := range fields {
		// Get the corresponding field using reflection
		fieldValue := userFields.FieldByName(field)
		if !fieldValue.IsValid() {
			return fmt.Errorf("invalid field: %s", field)
		}

		// Ensure the field is settable
		if !fieldValue.CanSet() {
			return fmt.Errorf("cannot set value of field: %s", field)
		}

		// Get the value to be stored
		val := fieldValue.Interface()
		// Update the database
		if err := st.db.UpdateField(existingUser, field, val); err != nil {
			return fmt.Errorf("failed to update user field: %s, error: %v", field, err)
		}
	}
	return nil
}

func (st usersBackend) Save(user *users.User, changePass bool) error {
	if settings.Config.Auth.Methods.PasswordAuth.Enabled && changePass {
		err := checkPassword(user.Password)
		if err != nil {
			return err
		}
		pass, err := users.HashPwd(user.Password)
		if err != nil {
			return err
		}
		user.Password = pass
	}

	// converting scopes to map of paths intead of names (names can change)
	adjustedScopes, err := settings.ConvertToBackendScopes(user.Scopes)
	if err != nil {
		return err
	}
	user.Scopes = adjustedScopes

	err = files.MakeUserDirs(user)
	if err != nil {
		logger.Error(err.Error())
	}
	err = st.db.Save(user)
	if err == storm.ErrAlreadyExists {
		return errors.ErrExist
	}
	return err
}

func (st usersBackend) DeleteByID(id uint) error {
	return st.db.DeleteStruct(&users.User{ID: id})
}

func (st usersBackend) DeleteByUsername(username string) error {
	user, err := st.GetBy(username)
	if err != nil {
		return err
	}

	return st.db.DeleteStruct(user)
}

// Define a function to check for restricted fields
func checkRestrictedFields(existingUser *users.User, fields []string) error {
	// Check if 'password' field is locked
	if existingUser.LockPassword && slices.Contains(fields, "password") {
		return fmt.Errorf("password is locked")
	}
	// Use reflection to get the field names of NonAdminEditable
	editableFields := reflect.ValueOf(existingUser)
	if editableFields.Kind() == reflect.Struct {
		for i := 0; i < editableFields.NumField(); i++ {
			fieldName := editableFields.Type().Field(i).Name
			if slices.Contains(fields, fieldName) {
				return fmt.Errorf("non-admins cannot modify field: %s", fieldName)
			}
		}
	}
	// No restricted fields found, return nil (no error)
	return nil
}

func parseFields(user *users.User, fields []string) ([]string, error) {
	// If `Which` is not specified, default to updating all fields
	if len(fields) == 0 || fields[0] == "all" {
		fields = []string{}
		v := reflect.ValueOf(user)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		t := v.Type()

		// Dynamically populate fields to update
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// which=all can't update password
			if strings.ToLower(field.Name) != "password" && strings.ToLower(field.Name) != "id" && strings.ToLower(field.Name) != "username" {
				fields = append(fields, field.Name)
			}
		}
	}
	newfields := []string{}
	for _, field := range fields {
		capitalField := utils.CapitalizeFirst(field)
		if capitalField == "Scopes" {
			newScopes, err := settings.ConvertToBackendScopes(user.Scopes)
			if err == nil {
				user.Scopes = newScopes
			}
			err = files.MakeUserDirs(user)
			if err != nil {
				logger.Error(err.Error())
			}
		}
		if capitalField == "Password" {
			value, err := users.HashPwd(user.Password)
			if err != nil {
				logger.Error(err.Error())
			}
			user.Password = value
		}
		newfields = append(newfields, capitalField)
	}

	return newfields, nil
}

func checkPassword(password string) error {
	if len(password) < settings.Config.Auth.Methods.PasswordAuth.MinLength {
		return fmt.Errorf("password must be at least %d characters long", settings.Config.Auth.Methods.PasswordAuth.MinLength)
	}
	return nil
}
