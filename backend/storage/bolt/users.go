package bolt

import (
	"fmt"
	"reflect"

	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/errors"
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

func (st usersBackend) Update(user *users.User, fields ...string) error {
	if len(fields) == 0 {
		return nil
	}

	userValue := reflect.ValueOf(user).Elem() // Get reflect.Value of user struct
	var target reflect.Value

	if user.Perm.Admin {
		target = userValue // Admins can update all fields
	} else {
		target = userValue.FieldByName("NonAdminEditable") // Non-admins can only update NonAdminEditable fields
		if !target.IsValid() {
			return fmt.Errorf("NonAdminEditable struct not found")
		}
	}

	for _, field := range fields {
		correctedField := utils.CapitalizeFirst(field)

		userField := target.FieldByName(correctedField)
		if !userField.IsValid() {
			return fmt.Errorf("invalid field: %s", field)
		}
		if !userField.CanSet() {
			return fmt.Errorf("cannot update unexported field: %s", field)
		}

		fieldValue := userField.Interface()
		if err := st.db.UpdateField(user, correctedField, fieldValue); err != nil {
			return fmt.Errorf("failed to update user field: %s, error: %v", correctedField, err)
		}
	}

	return nil
}

func (st usersBackend) Save(user *users.User) error {
	if settings.Config.Auth.Methods.PasswordAuth.Enabled {
		if len(user.Password) < settings.Config.Auth.Methods.PasswordAuth.MinLength {
			return fmt.Errorf("password must be at least %d characters long", settings.Config.Auth.Methods.PasswordAuth.MinLength)
		}
		pass, err := users.HashPwd(user.Password)
		if err != nil {
			return err
		}
		user.Password = pass
	}
	err := st.db.Save(user)
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
