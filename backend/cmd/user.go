package cmd

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
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
		if source.Config.CreateUserDir && !user.Perm.Admin {
			scopePath = fmt.Sprintf("%s%s", scopePath, users.CleanUsername(user.Username))
		}
		// if user doesn't yet have scope for source, add it for admins and default sources
		if err != nil {
			if user.Perm.Admin || source.Config.DefaultEnabled {
				newScopes = append(newScopes, users.SourceScope{Scope: scopePath, Name: source.Path}) // backend name is path
				updateUser = true
			}
		} else {
			newScopes = append(newScopes, users.SourceScope{Scope: scopePath, Name: source.Path}) // backend name is path
		}
	}

	// maintain backwards compatibility, update user scope from scopes
	if len(newScopes) == 0 {
		newScopes = []users.SourceScope{
			{
				Scope: settings.Config.Server.DefaultSource.Config.DefaultUserScope,
				Name:  settings.Config.Server.DefaultSource.Path, // backend name is path
			},
		}
		updateUser = true
	}
	user.Scopes = newScopes
	return updateUser
}
