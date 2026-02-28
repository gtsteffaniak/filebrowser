package cmd

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

var createBackup = false

func validateUserInfo(newDB bool) {
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
		if updateShowFirstLogin(user) {
			updateUser = true
		}
		if updateSidebarLinks(user) {
			updateUser = true
		}
		if updateTokens(user) {
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
			skipCreateBackup := os.Getenv("FILEBROWSER_DISABLE_AUTOMATIC_BACKUP") == "true" || newDB
			if createBackup && !skipCreateBackup {
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
		existingScope, ok := existing[src.Path]
		if ok {
			// If scope is empty and there's a default, apply default
			if existingScope.Scope == "" {
				existingScope.Scope = src.Config.DefaultUserScope
			}
		} else if src.Config.DefaultEnabled {
			existingScope.Scope = src.Config.DefaultUserScope
		} else {
			continue
		}

		newScopes = append(newScopes, users.SourceScope{
			Name:  src.Path,
			Scope: existingScope.Scope,
		})
		seen[src.Path] = struct{}{}
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

func updateShowFirstLogin(user *users.User) bool {
	if user.ShowFirstLogin && !settings.Env.IsFirstLoad {
		user.ShowFirstLogin = false
		return true
	}
	return false
}

// func to convert legacy user with perm key to permissions
func updatePermissions(user *users.User) bool {
	if user.Version >= 1 {
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
	user.Version = 2
	if updateUser {
		createBackup = true
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

// updateSidebarLinks checks if user has stale source links and rebuilds them if needed
func updateSidebarLinks(user *users.User) bool {
	// Count source links and check if any are still valid
	sourceLinksCount := 0
	validSourceLinksCount := 0

	for _, link := range user.SidebarLinks {
		if strings.HasPrefix(link.Category, "source") {
			sourceLinksCount++
			// Check if this source still exists
			if link.SourceName != "" {
				if _, ok := settings.Config.Server.SourceMap[link.SourceName]; ok {
					validSourceLinksCount++
				}
			}
		}
	}

	// If user has no source links, don't update anything
	if sourceLinksCount == 0 {
		return false
	}

	// If user has source links but NONE are valid, rebuild from their scopes
	if validSourceLinksCount == 0 {
		logger.Infof("User %s has %d stale source links, rebuilding from scopes", user.Username, sourceLinksCount)

		// Remove all existing source links
		newLinks := []users.SidebarLink{}
		for _, link := range user.SidebarLinks {
			if !strings.HasPrefix(link.Category, "source") {
				newLinks = append(newLinks, link)
			}
		}

		// Add new source links based on user's scopes
		for _, scope := range user.Scopes {
			if source, ok := settings.Config.Server.SourceMap[scope.Name]; ok {
				// User has access to this source, add it to sidebar
				newLinks = append(newLinks, users.SidebarLink{
					Name:       source.Name,
					Category:   "source",
					Target:     "/",
					Icon:       "",
					SourceName: source.Path,
				})
			}
		}

		user.SidebarLinks = newLinks
		return true
	}

	// User has at least one valid source link, no update needed
	return false
}

func updateTokens(user *users.User) bool {
	if user.Version >= 2 {
		return false
	}
	if user.ApiKeys != nil {
		user.Tokens = make(map[string]users.AuthToken)
		for name, token := range user.ApiKeys {
			token.Token = token.Key
			user.Tokens[name] = token
		}
	}
	user.Version = 2
	return true
}
