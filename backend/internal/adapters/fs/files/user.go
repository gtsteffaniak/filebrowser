package files

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
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
	settings.ExpandBackendScopesForCreateUserDir(u)
	for i, scope := range u.BackendScopes {
		source, ok := settings.Config.Server.SourceMap[scope.Path]
		if !ok {
			return fmt.Errorf("MakeUserDirs: source not found: %s", scope.Path)
		}
		fullPath := filepath.Join(source.Path, scope.Scope)
		parentDir := filepath.Dir(fullPath)
		if createDir {
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
		u.BackendScopes[i] = scope
	}
	return nil
}
