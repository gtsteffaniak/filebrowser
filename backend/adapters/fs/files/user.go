package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
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

func MakeUserDirs(u *users.User) error {
	cleanedUserName := users.CleanUsername(u.Username)
	if cleanedUserName == "" || cleanedUserName == "-" || cleanedUserName == "." {
		logger.Error(fmt.Sprintf("create user: invalid user for home dir creation: [%s]", u.Username))
	}
	for i, scope := range u.Scopes {
		source := settings.Config.Server.SourceMap[scope.Name]
		if source.Config.CreateUserDir {
			if !source.Config.CreateUserDir {
				continue
			}
			fullPath := ""
			if filepath.Base(scope.Scope) != cleanedUserName && !u.Perm.Admin {
				scope.Scope = filepath.Join(scope.Scope, cleanedUserName)
				fullPath = filepath.Join(source.Path, scope.Scope)
			} else {
				fullPath = filepath.Join(source.Path, scope.Scope, cleanedUserName)
			}
			err := MakeUserDir(fullPath)
			if err != nil {
				return fmt.Errorf("create user: failed to create user home dir: %s", err)
			}
			// update scope to reflect new user home dir
			u.Scopes[i] = scope
		}
	}
	return nil
}
