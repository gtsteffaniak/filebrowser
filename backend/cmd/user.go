package cmd

import (
	"fmt"
	"os"
	"reflect"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var createBackup = []bool{}

func validateUserInfo() {
	// update source info for users if names/sources/paths might have changed
	usersList, err := store.Users.Gets()
	if err != nil {
		logger.Fatalf("could not load users: %v", err)
	}
	for _, user := range usersList {
		changePass := false
		updateUser := false
		if updateUserScopes(user) {
			updateUser = true
		}
		if updatePermissions(user) {
			updateUser = true
		}
		if updatePreviewSettings(user) {
			updateUser = true
		}
		if updateLoginType(user) {
			updateUser = true
		}
		adminUser := settings.Config.Auth.AdminUsername
		adminPass := settings.Config.Auth.AdminPassword
		passwordEnabled := settings.Config.Auth.Methods.PasswordAuth.Enabled
		if user.Username == adminUser && adminPass != "" && passwordEnabled {
			logger.Info("Resetting admin user to default username and password.")
			user.Permissions.Admin = true
			user.Password = settings.Config.Auth.AdminPassword
			updateUser = true
			changePass = true
		}
		if updateUser {
			skipCreateBackup := os.Getenv("FILEBROWSER_DISABLE_AUTOMATIC_BACKUP") == "true"
			if len(createBackup) == 1 && !skipCreateBackup {
				logger.Warning("Incompatible user settings detected, creating backup of database before converting.")
				err = fileutils.CopyFile(settings.Config.Server.Database, fmt.Sprintf("%s.bak", settings.Config.Server.Database))
				if err != nil {
					logger.Fatalf("Unable to create automatic backup of database due to error: %v", err)
				}
			}
			err := store.Users.Save(user, changePass, true)
			if err != nil {
				logger.Errorf("could not update user: %v", err)
			}
		}
	}
}

func updateUserScopes(user *users.User) bool {
	newScopes := []users.SourceScope{}
	seen := make(map[string]struct{})

	// Build map for existing scopes by Name
	existing := make(map[string]users.SourceScope)
	for _, s := range user.Scopes {
		existing[s.Name] = s
	}

	// Preserve order by using Config.Server.Sources
	for _, src := range settings.Config.Server.Sources {
		realsource, ok := settings.Config.Server.NameToSource[src.Name]
		if !ok {
			continue
		}
		existingScope, ok := existing[realsource.Path]
		if ok {
			// If scope is empty and there's a default, apply default
			if existingScope.Scope == "" {
				existingScope.Scope = src.Config.DefaultUserScope
			}
		} else if realsource.Config.DefaultEnabled {
			existingScope.Scope = realsource.Config.DefaultUserScope
		} else {
			continue
		}

		newScopes = append(newScopes, users.SourceScope{
			Name:  realsource.Path,
			Scope: existingScope.Scope,
		})
		seen[realsource.Path] = struct{}{}
	}

	// Preserve user-defined scopes not matching current sources, append to end
	for _, s := range user.Scopes {
		if _, ok := seen[s.Name]; !ok {
			newScopes = append(newScopes, s)
		}
	}
	changed := !reflect.DeepEqual(user.Scopes, newScopes)
	user.Scopes = newScopes
	return changed
}

// func to convert legacy user with perm key to permissions
func updatePermissions(user *users.User) bool {
	if user.Version == 1 {
		return false
	}
	updateUser := true
	user.Permissions.Download = true
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
	if user.Perm.Create {
		user.Permissions.Create = true
		user.Perm.Create = false
		updateUser = true
	}
	if user.Perm.Create {
		user.Permissions.Create = true
		user.Perm.Create = false
		updateUser = true
	}
	if user.Perm.Download {
		user.Permissions.Download = true
		user.Perm.Download = false
		updateUser = true
	}
	if user.Permissions.Modify {
		user.Permissions.Create = true
		user.Permissions.Delete = true
		updateUser = true
	}
	user.Version = 1
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

func updatePreviewSettings(user *users.User) bool {
	// if user hasn't been updated yet
	if user.LoginMethod == "" {
		user.Preview.Image = true
		user.Preview.PopUp = true
		return true
	}
	return false
}
