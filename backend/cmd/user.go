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
		updateUserScopes(user)
	}
}

func updateUserScopes(user *users.User) {
	updateUser := false
	newScopes := []users.SourceScope{}
	for _, source := range settings.Config.Server.SourceMap {
		scopePath, err := settings.GetScopeFromSourcePath(user.Scopes, source.Path)
		if !user.Perm.Admin {
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
	if !updateUser {
		return
	}
	err := store.Users.Save(user, false)
	if err != nil {
		logger.Error(fmt.Sprintf("could not update user: %v", err))
	}
}
