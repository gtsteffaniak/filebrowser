package cmd

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

var createBackup = []bool{}

func validateUserInfo() {
	// update source info for users if names/sources/paths might have changed
	usersList, err := store.Users.Gets()
	if err != nil {
		logger.Fatal(fmt.Sprintf("could not load users: %v", err))
	}
	for _, user := range usersList {
		updateUser := false
		if user.Username == "publicUser" {
			settings.ApplyUserDefaults(user)
			updateUser = true
		}
		if updateUserScopes(user) {
			updateUser = true
		}
		if updatePermissions(user) {
			updateUser = true
		}
		if updateLoginType(user) {
			updateUser = true
		}
		if updateUser {
			if len(createBackup) == 1 {
				logger.Warning("Incompatible user settings detected, creating backup of database before converting.")
				err = fileutils.CopyFile(settings.Config.Server.Database, fmt.Sprintf("%s.bak", settings.Config.Server.Database))
				if err != nil {
					logger.Fatal(fmt.Sprintf("Unable to create automatic backup of database due to error: %v", err))
				}
			}
			err := store.Users.Save(user, false)
			if err != nil {
				logger.Error(fmt.Sprintf("could not update user: %v", err))
			}
		}
	}
}

func updateUserScopes(user *users.User) bool {
	updateUser := false
	newScopes := []users.SourceScope{}
	for _, source := range settings.Config.Server.SourceMap {
		scopePath, err := settings.GetScopeFromSourcePath(user.Scopes, source.Path)
		// apply default scope if it doesn't exist
		if !user.Perm.Admin && scopePath == "" {
			scopePath = source.Config.DefaultUserScope
		}
		if scopePath == "" {
			scopePath = "/"
		}
		if source.Config.CreateUserDir && !user.Permissions.Admin {
			scopePath = fmt.Sprintf("%s%s", scopePath, users.CleanUsername(user.Username))
		}
		// if user doesn't yet have scope for source, add it for admins and default sources
		if err != nil {
			if user.Permissions.Admin || source.Config.DefaultEnabled {
				newScopes = append(newScopes, users.SourceScope{Scope: scopePath, Name: source.Path}) // backend name is path
				updateUser = true
			}
		} else {
			newScopes = append(newScopes, users.SourceScope{Scope: scopePath, Name: source.Path}) // backend name is path
		}
	}

	// maintain backwards compatibility, update user scope from scopes
	if len(newScopes) == 0 {
		user.Scopes = []users.SourceScope{
			{
				Scope: settings.Config.Server.DefaultSource.Config.DefaultUserScope,
				Name:  settings.Config.Server.DefaultSource.Path, // backend name is path
			},
		}
		updateUser = true
	}
	return updateUser
}

// func to convert legacy user with perm key to permissions
func updatePermissions(user *users.User) bool {
	updateUser := false
	// if any keys are true, set the permissions to true
	if user.Perm.Api {
		user.Permissions.Api = true
		user.Perm.Api = false
		updateUser = true
	}
	if user.Perm.Admin {
		user.Permissions.Admin = true
		user.Perm.Admin = false
		updateUser = true
	}
	if user.Perm.Modify {
		user.Permissions.Modify = true
		user.Perm.Modify = false
		updateUser = true
	}
	if user.Perm.Share {
		user.Permissions.Share = true
		user.Perm.Share = false
		updateUser = true
	}
	if updateUser {
		createBackup = append(createBackup, true)
	}
	return updateUser
}

func updateLoginType(user *users.User) bool {
	if user.LoginMethod == "" {
		user.LoginMethod = users.LoginMethodPassword
		return true
	}
	return false
}
