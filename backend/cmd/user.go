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
		changedFields := make([]string, 0, 8)
		changePass := false

		if updateUserScopes(user) {
			changedFields = append(changedFields, "backendScopes")
		}
		if updatePermissions(user) {
			changedFields = append(changedFields, "permissions", "perm", "version")
		}
		if updateSourcePermissions(user) {
			changedFields = append(changedFields, "backendScopes", "backendSourcePermissions", "version")
		}
		if updatePreviewSettings(user) {
			changedFields = append(changedFields, "preview")
		}
		if updateLoginType(user) {
			changedFields = append(changedFields, "loginMethod")
		}
		if updateShowFirstLogin(user) {
			changedFields = append(changedFields, "showFirstLogin")
		}
		if updateSidebarLinks(user) {
			changedFields = append(changedFields, "sidebarLinks")
		}
		if updateTokens(user) {
			changedFields = append(changedFields, "tokens", "version")
		}
		if normalizeApiTokenPermissions(user) {
			changedFields = append(changedFields, "tokens")
		}
		if state.ApplyEnforcedSyncToUser(user) {
			changedFields = append(changedFields, settings.UserJSONFieldsForEnforcedSync()...)
		}
		if state.ApplyEnforcedSourcePermissionsSyncToUser(user) {
			changedFields = append(changedFields, "backendScopes", "backendSourcePermissions")
		}
		if user.Version < users.ProfileStorageVersion {
			user.Version = users.ProfileStorageVersion
			changedFields = append(changedFields, "version")
		}
		adminUser := settings.PasswordAdminUsername()
		if user.Username == adminUser && user.Permissions.Admin {
			adminPerms := settings.AdminPerms()
			if user.Permissions.Share != adminPerms.Share || user.Permissions.Api != adminPerms.Api {
				user.Permissions.Share = adminPerms.Share
				user.Permissions.Api = adminPerms.Api
				user.Permissions.Admin = true
				changedFields = append(changedFields, "permissions")
			}
		}
		if user.Username == adminUser && settings.PasswordAdminPassword() != "" && user.LoginMethod == users.LoginMethodPassword {
			logger.Info("Resetting admin user to default username and password.")
			user.Permissions = settings.AdminPerms()
			user.Password = settings.PasswordAdminPassword()
			user.LoginMethod = users.LoginMethodPassword
			changedFields = append(changedFields, "permissions", "password", "loginMethod")
			changePass = true
		}

		changedFields = dedupeFields(changedFields)
		if len(changedFields) == 0 {
			continue
		}

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
		if err := state.UpdateUser(user, plainPass, changedFields...); err != nil {
			logger.Errorf("could not update user: %v", err)
		}
	}
}

func dedupeFields(fields []string) []string {
	seen := make(map[string]struct{}, len(fields))
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if field == "" {
			continue
		}
		key := strings.ToLower(field)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, field)
	}
	return out
}

func updateUserScopes(user *users.User) bool {
	newScopes := settings.MergeDefaultEnabledBackendScopes(user.BackendScopes, user.DeclinedDefaultSources)
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

// updateSidebarLinks normalizes sidebar links and ensures scoped sources have sidebar entries.
func updateSidebarLinks(user *users.User) bool {
	needsEnsure := usersidebar.NeedsSidebarLinksFromScopes(user.SidebarLinks, user.BackendScopes)
	validBefore := usersidebar.ValidSourceSidebarLinkCount(user.SidebarLinks)

	links, changed := usersidebar.PrepareSidebarLinksForPersist(user.SidebarLinks, user.BackendScopes)
	if !changed {
		return false
	}

	user.SidebarLinks = links

	if needsEnsure {
		if validBefore == 0 && len(user.SidebarLinks) > 0 {
			logger.Infof("User %s has stale source sidebar links, merging missing links from scopes", user.Username)
		} else if validBefore == 0 {
			logger.Infof("User %s has no source sidebar links, building from scopes", user.Username)
		} else {
			logger.Infof("User %s is missing sidebar links for some scoped sources, merging from scopes", user.Username)
		}
	}

	return true
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
