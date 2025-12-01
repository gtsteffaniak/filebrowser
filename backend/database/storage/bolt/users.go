package bolt

import (
	"fmt"
	"reflect"
	"slices"
	"strings"

	storm "github.com/asdine/storm/v3"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

type usersBackend struct {
	db *storm.DB
}

// NewUsersBackend returns a users.StorageBackend backed by storm DB.
func NewUsersBackend(db *storm.DB) users.StorageBackend {
	return &usersBackend{db: db}
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
		arg = "ID"
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

	passwordUser := existingUser.LoginMethod == users.LoginMethodPassword
	enforcedOtp := settings.Config.Auth.Methods.PasswordAuth.EnforcedOtp
	if passwordUser && enforcedOtp && !user.OtpEnabled {
		return errors.ErrNoTotpConfigured
	}
	if user.LoginMethod == "" {
		user.LoginMethod = existingUser.LoginMethod
	}
	fields, err = parseFields(user, fields, actorIsAdmin)
	if err != nil {
		return err
	}

	if user.LoginMethod == "" {
		user.LoginMethod = existingUser.LoginMethod
	}

	if !slices.Contains(fields, "Password") {
		user.Password = existingUser.Password
	} else {
		if existingUser.LockPassword && !actorIsAdmin {
			return fmt.Errorf("password cannot be changed when lock password is enabled")
		}
	}

	if !actorIsAdmin {
		fields = filterRestrictedFields(fields)
	}

	if len(fields) == 0 {
		return fmt.Errorf("no fields to update")
	}

	// converting scopes to map of paths intead of names (names can change)
	if slices.Contains(fields, "Scopes") {
		adjustedScopes, err := settings.ConvertToBackendScopes(user.Scopes)
		if err != nil {
			return err
		}
		user.Scopes = adjustedScopes
		files.MakeUserDirs(user, true)
	}
	// converting scopes to map of paths intead of names (names can change)
	if slices.Contains(fields, "SidebarLinks") {
		adjustedLinks, err := settings.ConvertToBackendSidebarLinks(user.SidebarLinks)
		if err != nil {
			return err
		}
		user.SidebarLinks = adjustedLinks
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
		if field == "OtpEnabled" {
			otpEnabled, _ := val.(bool)
			if !otpEnabled {
				field = "TOTPSecret" // if otp is disabled, we also want to clear the TOTPSecret
				val = ""             // clear the TOTPSecret
			}
			// If otpEnabled is true, continue with normal field update
		}
		// Update the database
		if err := st.db.UpdateField(existingUser, field, val); err != nil {
			return fmt.Errorf("failed to update user field: %s, error: %v", field, err)
		}
	}

	// last revoke api keys if needed.
	if existingUser.Permissions.Api && !user.Permissions.Api && slices.Contains(fields, "Permissions") {
		for _, key := range existingUser.ApiKeys {
			auth.RevokeAPIKey(key.Key) // add to blacklist
		}
	}
	return nil
}

func (st usersBackend) Save(user *users.User, changePass, disableScopeChange bool) error {
	if user.LoginMethod == "" {
		user.LoginMethod = users.LoginMethodPassword
	}
	if user.Username == "anonymous" {
		return fmt.Errorf("username cannot be 'anonymous'")
	}
	logger.Debugf("Saving user [%s] changepass: %v", user.Username, changePass)
	if user.LoginMethod == users.LoginMethodPassword && changePass {
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
	files.MakeUserDirs(user, disableScopeChange)
	err = st.db.Save(user)
	if err == storm.ErrAlreadyExists {
		return fmt.Errorf("user with provided username already exists")
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

// Define a function to filter out restricted fields for non-admin users
func filterRestrictedFields(fields []string) []string {
	// Get a list of allowed fields from NonAdminEditable
	allowed := getNonAdminEditableFieldNames()
	var filteredFields []string

	for _, field := range fields {
		if slices.Contains(allowed, field) {
			filteredFields = append(filteredFields, field)
		}
	}

	return filteredFields
}

// Helper to return list of field names from NonAdminEditable struct
func getNonAdminEditableFieldNames() []string {
	var names []string
	t := reflect.TypeOf(users.NonAdminEditable{})
	for i := 0; i < t.NumField(); i++ {
		names = append(names, t.Field(i).Name)
	}
	return names
}

func parseFields(user *users.User, fields []string, actorIsAdmin bool) ([]string, error) {
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
			switch strings.ToLower(field.Name) {
			case "id", "username", "password", "apikeys", "totpsecret", "totpnonce":
				// Skip these fields
				continue
			}

			// Handle embedded structs (like NonAdminEditable)
			if field.Anonymous {
				// Get the embedded struct type
				embeddedType := field.Type
				if embeddedType.Kind() == reflect.Ptr {
					embeddedType = embeddedType.Elem()
				}

				// Add all fields from the embedded struct
				for j := 0; j < embeddedType.NumField(); j++ {
					embeddedField := embeddedType.Field(j)
					fields = append(fields, embeddedField.Name)
				}
			} else {
				fields = append(fields, field.Name)
			}
		}
	}
	newfields := []string{}
	for _, field := range fields {
		capitalField := utils.CapitalizeFirst(field)
		if capitalField == "Scopes" {
			if !actorIsAdmin {
				continue
			}
		}
		if capitalField == "Password" {
			// Only process password if it's actually being updated (not empty)
			if user.Password != "" {
				logger.Debugf("Updating password for user [%s] loginMethod: %s", user.Username, user.LoginMethod)
				if user.LoginMethod != users.LoginMethodPassword {
					return nil, fmt.Errorf("password cannot be changed when login method is not password")
				}
				err := checkPassword(user.Password)
				if err != nil {
					return nil, fmt.Errorf("password does not meet complexity requirements")
				}
				value, err := users.HashPwd(user.Password)
				if err != nil {
					logger.Error(err.Error())
				}
				user.Password = value
			} else {
				// Skip password field if it's empty
				continue
			}
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
