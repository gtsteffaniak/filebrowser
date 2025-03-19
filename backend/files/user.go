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
				stringErr := fmt.Sprintf("create user: failed to get index for user home dir creation: %s", source.Name)
				logger.Error(stringErr)
				return fmt.Errorf(stringErr)
			}
			if !idx.Config.CreateUserDir || u.Perm.Admin {
				continue
			}
			if filepath.Base(scope.Scope) != cleanedUserName {
				scope.Scope = filepath.Join(scope.Scope, cleanedUserName)
			}
			fullPath := filepath.Join(source.Path, scope.Scope)
			err := idx.MakeUserDir(fullPath)
			if err != nil {
				stringErr := fmt.Sprintf("create user: failed to create user home dir: %s", err)
				logger.Error(stringErr)
				return fmt.Errorf(stringErr)
			}
			// update scope to reflect new user home dir
			u.Scopes[i] = scope
		}
	}
	return nil
}
