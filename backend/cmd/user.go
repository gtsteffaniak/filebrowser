package cmd

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

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
		if updateUser {
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
		if !user.Permissions.Admin {
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
		user.Scopes = newScopes
	}

	// maintain backwards compatibility, update user scope from scopes
	if len(user.Scopes) == 0 {
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
		updateUser = true
	}
	if user.Perm.Admin {
		user.Permissions.Admin = true
		updateUser = true
	}
	if user.Perm.Modify {
		user.Permissions.Modify = true
		updateUser = true
	}
	if user.Perm.Share {
		user.Permissions.Share = true
		updateUser = true
	}
	return updateUser
}
