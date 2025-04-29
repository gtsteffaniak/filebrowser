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
		if updatePreviewSettings(user) {
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
	finalScopes := []users.SourceScope{}
	seenNames := make(map[string]bool)

	// Step 1: Start by including all existing scopes, updating Name if source.Path matches
	for _, existingScope := range user.Scopes {
		updated := false
		for _, source := range settings.Config.Server.SourceMap {
			if existingScope.Name == source.Path || existingScope.Scope == source.Path {
				// Update Name to match source.Path
				existingScope.Name = source.Path
				updated = true
				break
			}
		}
		if existingScope.Scope == "" {
			existingScope.Scope = "/"
			updateUser = true
		}
		if !seenNames[existingScope.Name] {
			finalScopes = append(finalScopes, existingScope)
			seenNames[existingScope.Name] = true
			if updated {
				updateUser = true
			}
		}
	}

	// Step 2: Add missing scopes from SourceMap if admin or DefaultEnabled
	for _, source := range settings.Config.Server.SourceMap {
		if seenNames[source.Path] {
			continue
		}

		// Check if scope exists already via path
		scopePath, err := settings.GetScopeFromSourcePath(user.Scopes, source.Path)

		// If user has no access, skip unless admin or default-enabled
		if err != nil && !(user.Permissions.Admin || source.Config.DefaultEnabled) {
			continue
		}

		// Determine scope path
		if scopePath == "" {
			scopePath = source.Config.DefaultUserScope
		}
		if scopePath == "" {
			scopePath = "/"
		}
		if source.Config.CreateUserDir && !user.Permissions.Admin {
			scopePath = fmt.Sprintf("%s%s", scopePath, users.CleanUsername(user.Username))
		}

		// Add new scope
		finalScopes = append(finalScopes, users.SourceScope{
			Scope: scopePath,
			Name:  source.Path,
		})
		seenNames[source.Path] = true
		updateUser = true
	}

	// Step 3: If no scopes, apply default
	if len(finalScopes) == 0 {
		defaultScope := users.SourceScope{
			Scope: settings.Config.Server.DefaultSource.Config.DefaultUserScope,
			Name:  settings.Config.Server.DefaultSource.Path,
		}
		finalScopes = append(finalScopes, defaultScope)
		updateUser = true
	}

	// Step 4: Assign only if different
	if !scopesEqual(user.Scopes, finalScopes) {
		user.Scopes = finalScopes
		updateUser = true
	}

	return updateUser
}

// scopesEqual does a deep comparison to avoid unnecessary writes
func scopesEqual(a, b []users.SourceScope) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Scope != b[i].Scope || a[i].Name != b[i].Name {
			return false
		}
	}
	return true
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

func updatePreviewSettings(user *users.User) bool {
	// if user hasn't been updated yet
	if user.LoginMethod == "" {
		user.Preview.Image = true
		user.Preview.PopUp = true
		return true
	}
	return false
}
