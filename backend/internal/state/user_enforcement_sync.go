package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// ApplyEnforcedSyncToUser applies enforced default values onto u in memory. Returns true if profile fields changed.
func ApplyEnforcedSyncToUser(u *users.User) bool {
	if u == nil {
		return false
	}
	d := EffectiveUserDefaults()
	e := EffectiveEnforced()
	return settings.SyncEnforcedDefaultsOntoUser(u, d, e)
}

// ResyncEnforcedDefaultsForAllUsers writes enforced default values into SQLite for every user when they differ.
func ResyncEnforcedDefaultsForAllUsers() error {
	userDefaultsMu.RLock()
	defer userDefaultsMu.RUnlock()

	usersMux.Lock()
	defer usersMux.Unlock()

	d := userDefaultsDefault
	e := userDefaultsEnforcedDefault

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for enforced sync: %w", err)
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
		if !settings.SyncEnforcedDefaultsOntoUser(u, d, e) {
			continue
		}
		users.SyncBackendSourcePermissionsMap(u)
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync enforced defaults for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced enforced user defaults for %d users", updated)
	}
	return nil
}
