package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// ApplyEnforcedSourcePermissionsSyncToUser applies enforced source permission defaults onto u in memory.
// Returns true if scope permissions changed.
func ApplyEnforcedSourcePermissionsSyncToUser(u *users.User) bool {
	if u == nil {
		return false
	}
	defaults := GetSourceAccessDefaults()
	enforced := GetEnforcedSourcePermissions()
	return settings.SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced)
}

// ResyncEnforcedSourcePermissionsForAllUsers writes enforced source permission defaults into SQLite for every user when they differ.
func ResyncEnforcedSourcePermissionsForAllUsers() error {
	sourceAccessMu.RLock()
	defer sourceAccessMu.RUnlock()

	usersMux.Lock()
	defer usersMux.Unlock()

	defaults := GetSourceAccessDefaults()
	enforced := sourceAccessEnforcedDefault

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for source permission enforced sync: %w", err)
	}

	var updated int
	for _, row := range usersList {
		if row == nil {
			continue
		}
		u := cloneUserPtr(row)
		if !settings.EnforcementAppliesToUser(u) {
			continue
		}
		if !settings.SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced) {
			continue
		}
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync enforced source permissions for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced enforced source permissions for %d users", updated)
	}
	return nil
}
