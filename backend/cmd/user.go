package cmd

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/go-logger/logger"
)

var createBackup = false

func validateUserInfo(newDB bool) {
	// update source info for users if names/sources/paths might have changed
	usersList, err := state.GetAllUsers()
	if err != nil {
		logger.Fatalf("could not load users: %v", err)
	}
	for i := range usersList {
		user := &usersList[i]
		updateUser := false
		changePass := false
		if updateUserScopes(user) {
			updateUser = true
		}
		if updatePermissions(user) {
			updateUser = true
		}
		if updateSourcePermissions(user) {
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
		if normalizeApiTokenPermissions(user) {
			updateUser = true
		}
		if state.ApplyEnforcedSyncToUser(user) {
			updateUser = true
		}
		if user.Version < users.ProfileStorageVersion {
			user.Version = users.ProfileStorageVersion
			updateUser = true
		}
		adminUser := settings.Config.Auth.AdminUsername
		if adminUser == "" {
			adminUser = "admin"
		}
		adminPass := settings.Config.Auth.AdminPassword
		if adminPass == "" {
			adminPass = "admin"
		}
		if user.Username == adminUser && user.Permissions.Admin {
			adminPerms := settings.AdminPerms()
			if user.Permissions.Share != adminPerms.Share || user.Permissions.Api != adminPerms.Api {
				user.Permissions.Share = adminPerms.Share
				user.Permissions.Api = adminPerms.Api
				user.Permissions.Admin = true
				updateUser = true
			}
		}
		if user.Username == adminUser && adminPass != "" && user.LoginMethod == users.LoginMethodPassword {
			logger.Info("Resetting admin user to default username and password.")
			user.Permissions = settings.AdminPerms()
			user.Password = settings.Config.Auth.AdminPassword
			updateUser = true
			changePass = true
		}
		if updateUser {
			skipCreateBackup := os.Getenv("FILEBROWSER_DISABLE_AUTOMATIC_BACKUP") == "true" || newDB
			if createBackup && !skipCreateBackup {
				logger.Warning("Incompatible user settings detected, creating backup of database before converting.")
				err = fileutils.CopyFile(settings.Config.Server.DatabaseV2.Path, fmt.Sprintf("%s.bak", settings.Config.Server.DatabaseV2.Path))
				if err != nil {
					logger.Fatalf("Unable to create automatic backup of database due to error: %v", err)
				}
			}
			plainPass := ""
			if changePass {
				plainPass = user.Password
			}
			// Full update: migration may touch enforced profile fields beyond the legacy whitelist.
			err := state.UpdateUser(user, plainPass)
			if err != nil {
				logger.Errorf("could not update user: %v", err)
			}
		}

	}
}

func updateUserScopes(user *users.User) bool {
	newScopes := []users.BackendScope{}
	seen := make(map[string]struct{})

	// Build map for existing scopes by Name
	existing := make(map[string]users.BackendScope)
	for _, s := range user.BackendScopes {
		existing[s.Path] = s
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
		newScopes = append(newScopes, users.BackendScope{
			Path:        src.Path,
			Scope:       existingScope.Scope,
			Permissions: existingScope.Permissions,
		})
		seen[src.Path] = struct{}{}
	}

	// Preserve user-defined scopes not matching current sources, append to end
	for _, s := range user.BackendScopes {
		if _, ok := seen[s.Path]; !ok {
			newScopes = append(newScopes, s)
		}
	}
	changed := !reflect.DeepEqual(user.BackendScopes, newScopes)
	user.BackendScopes = newScopes

	return changed
}

func updateSourcePermissions(user *users.User) bool {
	changed := false
	if user.Version < users.SourcePermissionsMigrationVersion {
		if users.MigrateToSourcePermissions(user) {
			changed = true
		}
	}
	if users.EnsureSourcePermissionsForScopes(
		user,
		settings.DefaultSourceFilePermissions(),
		settings.AdminSourceFilePermissions(),
	) {
		changed = true
	}
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

// updateSidebarLinks normalizes sidebar links and rebuilds from scopes when none resolve.
func updateSidebarLinks(user *users.User) bool {
	updated := false

	if normalized, changed := usersidebar.NormalizeSidebarLinks(user.SidebarLinks); changed {
		user.SidebarLinks = normalized
		updated = true
	}

	sourceLinksCount, validSourceLinksCount := countSidebarSourceLinks(user.SidebarLinks)

	shouldRebuildFromScopes := len(user.BackendScopes) > 0 &&
		(sourceLinksCount == 0 || validSourceLinksCount == 0)
	if !shouldRebuildFromScopes {
		return updated
	}

	if sourceLinksCount > 0 && validSourceLinksCount == 0 {
		logger.Infof("User %s has %d stale source links, rebuilding from scopes", user.Username, sourceLinksCount)
	} else if sourceLinksCount == 0 {
		logger.Infof("User %s has no source sidebar links, building from scopes", user.Username)
	}

	newLinks := []users.SidebarLink{}
	for _, link := range user.SidebarLinks {
		if !strings.HasPrefix(link.Category, "source") {
			newLinks = append(newLinks, link)
		}
	}

	for _, scope := range user.BackendScopes {
		if source, ok := settings.Config.Server.SourceMap[scope.Path]; ok {
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
	if normalized, changed := usersidebar.NormalizeSidebarLinks(user.SidebarLinks); changed {
		user.SidebarLinks = normalized
	}
	return true
}

func countSidebarSourceLinks(links []users.SidebarLink) (total, valid int) {
	for _, link := range links {
		if !strings.HasPrefix(link.Category, "source") {
			continue
		}
		total++
		if link.SourceName != "" {
			if _, ok := users.ResolveSourceKey(link.SourceName); ok {
				valid++
				continue
			}
		}
		if link.Name != "" {
			if _, ok := users.ResolveSourceKey(link.Name); ok {
				valid++
			}
		}
	}
	return total, valid
}

func updateTokens(user *users.User) bool {
	if user.Version >= 2 {
		return false
	}
	if user.ApiKeys != nil {
		user.Tokens = make(map[string]users.AuthToken)
		for name, token := range user.ApiKeys {
			token.Token = token.Key
			token.Name = name
			users.StoreToken(user.Tokens, token)
		}
	}
	user.Version = 2
	return true
}

func normalizeApiTokenPermissions(user *users.User) bool {
	if user == nil || len(user.Tokens) == 0 {
		return false
	}
	changed := false
	for name, token := range user.Tokens {
		if token.Name == "" || name != token.Name {
			continue
		}
		sanitized := users.SanitizeTokenPermissions(token.Permissions)
		if sanitized != token.Permissions {
			token.Permissions = sanitized
			user.Tokens[name] = token
			changed = true
		}
	}
	return changed
}
