package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// MakeUserDir makes the user directory according to settings.
func MakeUserDir(fullPath string) error {
	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}
	return nil
}

func MakeUserDirs(u *users.User, createDir bool) error {
	cleanedUserName := users.CleanUsername(u.Username)
	if cleanedUserName == "" || cleanedUserName == "-" || cleanedUserName == "." {
		return fmt.Errorf("MakeUserDirs: invalid user for home dir creation: [%s]", u.Username)
	}
	for i, scope := range u.Scopes {
		source, ok := settings.Config.Server.SourceMap[scope.Name]
		if !ok {
			return fmt.Errorf("MakeUserDirs: source not found: %s", scope.Name)
		}
		fullPath := filepath.Join(source.Path, scope.Scope)
		parentDir := filepath.Dir(fullPath)
		if createDir {
			if source.Config.CreateUserDir && filepath.Base(scope.Scope) != cleanedUserName {
				scope.Scope = utils.JoinPathAsUnix(scope.Scope, cleanedUserName)
				fullPath = filepath.Join(fullPath, cleanedUserName)
			}
			if !Exists(parentDir) {
				if err := MakeUserDir(parentDir); err != nil {
					logger.Errorf("MakeUserDirs: failed to create parent scope directory: %s - %v", scope.Scope, err)
					continue
				}
			}
			err := MakeUserDir(fullPath)
			if err != nil {
				return fmt.Errorf("MakeUserDirs: failed to create user home dir: %s", err)
			}
		} else {
			if !Exists(fullPath) {
				return fmt.Errorf("MakeUserDirs: scope folder does not exist: %s", fullPath)
			}
		}

		u.Scopes[i] = scope
	}
	return nil
}
