package files

import (
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

func MakeUserDirs(u *users.User, disableScopeChange bool) {
	cleanedUserName := users.CleanUsername(u.Username)
	if cleanedUserName == "" || cleanedUserName == "-" || cleanedUserName == "." {
		logger.Errorf("MakeUserDirs: invalid user for home dir creation: [%s]", u.Username)
		return
	}
	for i, scope := range u.Scopes {
		source, ok := settings.Config.Server.SourceMap[scope.Name]
		if !ok {
			logger.Errorf("MakeUserDirs: source not found: %s", scope.Name)
			continue
		}
		// create directory and append user name
		if filepath.Base(scope.Scope) != cleanedUserName && source.Config.CreateUserDir && !disableScopeChange {
			fullPath := filepath.Join(source.Path, scope.Scope, cleanedUserName)
			parentDir := filepath.Join(source.Path, scope.Scope)
			// If parent directory doesn't exist and createUserDir is enabled, create it
			if !Exists(parentDir) {
				if err := MakeUserDir(parentDir); err != nil {
					logger.Errorf("MakeUserDirs: failed to create parent scope directory: %s - %v", scope.Scope, err)
					continue
				}
			}
			// Use JoinPathAsUnix to ensure scope remains in Unix format (forward slashes)
			scope.Scope = utils.JoinPathAsUnix(scope.Scope, cleanedUserName)
			err := MakeUserDir(fullPath)
			if err != nil {
				logger.Errorf("MakeUserDirs: failed to create user home dir: %s", err)
				// Continue to next scope even if this one failed
			}
		} else if filepath.Base(scope.Scope) == cleanedUserName && source.Config.CreateUserDir {
			// create directory exactly as specified
			fullPath := filepath.Join(source.Path, scope.Scope)
			parentDir := filepath.Dir(fullPath)
			if !Exists(parentDir) {
				logger.Errorf("MakeUserDirs: scope folder does not exist: %s", parentDir)
				continue
			}
			err := MakeUserDir(fullPath)
			if err != nil {
				logger.Errorf("create user: failed to create user home dir: %s", err)
				// Continue to next scope even if this one failed
			}
		} else {
			// just assigning scope to path provided, so just check that it exists
			path := filepath.Join(source.Path, scope.Scope)
			if !Exists(path) {
				logger.Errorf("MakeUserDirs: scope folder does not exist: %s", path)
				continue
			}
		}
		u.Scopes[i] = scope
	}
}
