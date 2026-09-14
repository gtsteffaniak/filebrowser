package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/toolaccess"
	"github.com/gtsteffaniak/go-logger/logger"
)

func resyncToolAccessForAllUsers(doc toolaccess.ToolAccessDefaultsDocument) error {
	if errInjectResyncToolAccessDefaults != nil {
		return errInjectResyncToolAccessDefaults
	}

	usersMux.Lock()
	defer usersMux.Unlock()

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for tool access sync: %w", err)
	}

	var updated int
	for _, row := range usersList {
		if row == nil {
			continue
		}
		u := cloneUserPtr(row)
		changed := toolaccess.MergeDefaultToolAccess(u, doc)
		if !u.Permissions.Admin {
			if toolaccess.MergeEnforcedToolAccess(u, doc) {
				changed = true
			}
		}
		if !changed {
			continue
		}
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync tool access for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced tool access for %d users", updated)
	}
	return nil
}

// ResyncToolAccessDefaultsForAllUsers applies current defaults and enforced policy to all users.
func ResyncToolAccessDefaultsForAllUsers() error {
	return resyncToolAccessForAllUsers(EffectiveToolAccessDefaults())
}
