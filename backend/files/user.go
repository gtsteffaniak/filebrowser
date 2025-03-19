package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

// todo fix this!!
// MakeUserDir makes the user directory according to settings.
func (idx *Index) MakeUserDir(fullPath string) error {
	logger.Debug(fmt.Sprintf("creating user home dir: [%s]", fullPath))
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
			idx := GetIndex(source.Name)
			if idx == nil {
				return fmt.Errorf("create user: failed to get index for user home dir creation: %s", source.Name)
			}
			if !idx.Config.CreateUserDir {
				continue
			}
			fullPath := ""
			if filepath.Base(scope.Scope) != cleanedUserName && !u.Perm.Admin {
				scope.Scope = filepath.Join(scope.Scope, cleanedUserName)
				fullPath = filepath.Join(source.Path, scope.Scope)
			} else {
				fullPath = filepath.Join(source.Path, scope.Scope, cleanedUserName)
			}
			err := idx.MakeUserDir(fullPath)
			if err != nil {
				return fmt.Errorf("create user: failed to create user home dir: %s", err)
			}
			// update scope to reflect new user home dir
			u.Scopes[i] = scope
		}
	}
	return nil
}
