package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// MakeUserDir makes the user directory according to settings.
func MakeUserDir(fullPath string) error {
	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}
	return nil
}

func MakeUserDirs(u *users.User, disableScopeChange bool) error {
	cleanedUserName := users.CleanUsername(u.Username)
	if cleanedUserName == "" || cleanedUserName == "-" || cleanedUserName == "." {
		return fmt.Errorf("create user: invalid user for home dir creation: [%s]", u.Username)
	}
	for i, scope := range u.Scopes {
		source, ok := settings.Config.Server.SourceMap[scope.Name]
		if !ok {
			return fmt.Errorf("create user: source not found: %s", scope.Name)
		}
		// create directory and append user name
		if filepath.Base(scope.Scope) != cleanedUserName && source.Config.CreateUserDir && !disableScopeChange {
			fullPath := filepath.Join(source.Path, scope.Scope, cleanedUserName)
			parentDir := filepath.Join(source.Path, scope.Scope)
			// validate that scope path exists
			if !Exists(parentDir) {
				return fmt.Errorf("create user: scope path does not exist: %s", scope.Scope)
			}
			scope.Scope = filepath.Join(scope.Scope, cleanedUserName)
			err := MakeUserDir(fullPath)
			if err != nil {
				return fmt.Errorf("create user: failed to create user home dir: %s", err)
			}
		} else if filepath.Base(scope.Scope) == cleanedUserName && source.Config.CreateUserDir {
			// create directory exactly as specified
			fullPath := filepath.Join(source.Path, scope.Scope)
			parentDir := filepath.Dir(fullPath)
			if !Exists(parentDir) {
				return fmt.Errorf("create user: scope folder does not exist: %s", parentDir)
			}
			err := MakeUserDir(fullPath)
			if err != nil {
				return fmt.Errorf("create user: failed to create user home dir: %s", err)
			}
		} else {
			// just assigning scope to path provided, so just check that it exists
			path := filepath.Join(source.Path, scope.Scope)
			if !Exists(path) {
				return fmt.Errorf("create user: scope folder does not exist: %s", path)
			}
		}
		u.Scopes[i] = scope
	}
	return nil
}
