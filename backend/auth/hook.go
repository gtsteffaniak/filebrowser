package auth

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

type hookCred struct {
	Password string `json:"password"`
	Username string `json:"username"`
}

// HookAuth is a hook implementation of an Auther.
type HookAuth struct {
	Users    users.Store        `json:"-"`
	Settings *settings.Settings `json:"-"`
	Server   *settings.Server   `json:"-"`
	Cred     hookCred           `json:"-"`
	Fields   hookFields         `json:"-"`
	Command  string             `json:"command"`
}

// Auth authenticates the user via a json in content body.
func (a *HookAuth) Auth(r *http.Request, usr *users.Storage) (*users.User, error) {
	var cred hookCred
	if r.Body == nil {
		return nil, os.ErrPermission
	}

	err := json.NewDecoder(r.Body).Decode(&cred)
	if err != nil {
		logger.Error("decode body error")
		return nil, os.ErrPermission
	}

	a.Users = usr
	a.Settings = &settings.Config
	a.Server = &settings.Config.Server
	a.Cred = cred

	action, err := a.RunCommand()
	if err != nil {
		return nil, err
	}
	logger.Debugf("hook auth %v", action)

	switch action {
	case "auth":
		u, err := a.SaveUser()
		if err != nil {
			return nil, err
		}
		return u, nil
	case "block":
		logger.Error("block error")

		return nil, os.ErrPermission
	case "pass":
		logger.Error("pass error")

		u, err := a.Users.Get(a.Cred.Username)
		if err != nil {
			return nil, fmt.Errorf("unable to get user from store: %v", err)
		}
		err = users.CheckPwd(cred.Password, u.Password)
		if err != nil {
			return nil, err
		}
		return u, nil
	default:
		return nil, fmt.Errorf("invalid hook action: %s", action)
	}
}

// LoginPage tells that hook auth requires a login page.
func (a *HookAuth) LoginPage() bool {
	return true
}

// RunCommand starts the hook command and returns the action
func (a *HookAuth) RunCommand() (string, error) {
	command := strings.Split(a.Command, " ")
	envMapping := func(key string) string {
		switch key {
		case "USERNAME":
			return a.Cred.Username
		case "PASSWORD":
			return a.Cred.Password
		default:
			return os.Getenv(key)
		}
	}
	for i, arg := range command {
		if i == 0 {
			continue
		}
		command[i] = os.Expand(arg, envMapping)
	}

	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec
	cmd.Env = append(os.Environ(), fmt.Sprintf("USERNAME=%s", a.Cred.Username))
	cmd.Env = append(cmd.Env, fmt.Sprintf("PASSWORD=%s", a.Cred.Password))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	a.GetValues(string(out))

	return a.Fields.Values["hook.action"], nil
}

// GetValues creates a map with values from the key-value format string
func (a *HookAuth) GetValues(s string) {
	m := map[string]string{}

	// make line breaks consistent on Windows platform
	s = strings.ReplaceAll(s, "\r\n", "\n")

	// iterate input lines
	for _, val := range strings.Split(s, "\n") {
		v := strings.SplitN(val, "=", 2) //nolint: gomnd

		// skips non key and value format
		if len(v) != 2 { //nolint: gomnd
			continue
		}

		fieldKey := strings.TrimSpace(v[0])
		fieldValue := strings.TrimSpace(v[1])

		if a.Fields.IsValid(fieldKey) {
			m[fieldKey] = fieldValue
		}
	}

	a.Fields.Values = m
}

// SaveUser updates the existing user or creates a new one when not found
func (a *HookAuth) SaveUser() (*users.User, error) {
	u, err := a.Users.Get(a.Cred.Username)
	if err != nil && err != errors.ErrNotExist {
		return nil, err
	}

	if u == nil {
		// create user with the provided credentials
		d := &users.User{
			NonAdminEditable: users.NonAdminEditable{
				Password:    a.Cred.Password,
				Locale:      a.Settings.UserDefaults.Locale,
				ViewMode:    a.Settings.UserDefaults.ViewMode,
				SingleClick: a.Settings.UserDefaults.SingleClick,
				ShowHidden:  a.Settings.UserDefaults.ShowHidden,
			},
			Username:    a.Cred.Username,
			Permissions: a.Settings.UserDefaults.Permissions,
		}
		u = a.GetUser(d)

		err = a.Users.Save(u, false, false)
		if err != nil {
			return nil, err
		}
		return u, nil
	}
	err = users.CheckPwd(a.Cred.Password, u.Password)
	if err != nil {
		return nil, err
	}

	if len(a.Fields.Values) > 1 {
		u = a.GetUser(u)
		// update user with provided fields
		err := a.Users.Update(u, u.Permissions.Admin)
		if err != nil {
			return nil, err
		}
	}

	return u, nil
}

// GetUser returns a User filled with hook values or provided defaults
func (a *HookAuth) GetUser(d *users.User) *users.User {
	// adds all permissions when user is admin
	isAdmin := d.Permissions.Admin
	perms := users.Permissions{
		Admin:  isAdmin,
		Modify: isAdmin || d.Permissions.Modify,
		Share:  isAdmin || d.Permissions.Share,
	}
	user := users.User{
		NonAdminEditable: users.NonAdminEditable{
			Password:    d.Password,
			Locale:      a.Fields.GetString("user.locale", d.Locale),
			ViewMode:    a.Fields.GetString("user.viewMode", d.ViewMode),
			SingleClick: a.Fields.GetBoolean("user.singleClick", d.SingleClick),
			ShowHidden:  a.Fields.GetBoolean("user.showHidden", d.ShowHidden),
		},
		ID:           d.ID,
		Username:     d.Username,
		Scopes:       d.Scopes,
		Permissions:  perms,
		LockPassword: true,
	}

	return &user
}

// hookFields is used to access fields from the hook
type hookFields struct {
	Values map[string]string
}

// validHookFields contains names of the fields that can be used
var validHookFields = []string{
	"hook.action",
	"user.scope",
	"user.locale",
	"user.viewMode",
	"user.singleClick",
	"user.sorting.by",
	"user.sorting.asc",
	"user.showHidden",
	"user.perm.admin",
	"user.perm.modify",
	"user.perm.share",
	"user.perm.api",
}

// IsValid checks if the provided field is on the valid fields list
func (hf *hookFields) IsValid(field string) bool {
	for _, val := range validHookFields {
		if field == val {
			return true
		}
	}

	return false
}

// GetString returns the string value or provided default
func (hf *hookFields) GetString(k, dv string) string {
	val, ok := hf.Values[k]
	if ok {
		return val
	}
	return dv
}

// GetBoolean returns the bool value or provided default
func (hf *hookFields) GetBoolean(k string, dv bool) bool {
	val, ok := hf.Values[k]
	if ok {
		return val == "true"
	}
	return dv
}

// GetArray returns the array value or provided default
func (hf *hookFields) GetArray(k string, dv []string) []string {
	val, ok := hf.Values[k]
	if ok && strings.TrimSpace(val) != "" {
		return strings.Split(val, " ")
	}
	return dv
}
