package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/usersidebar"
	"github.com/gtsteffaniak/go-logger/logger"
)

// ResyncSidebarLinkDefaultsForAllUsers merges enabled default sidebar links for every user.
func ResyncSidebarLinkDefaultsForAllUsers() error {
	return resyncSidebarLinkDefaultsForAllUsers(EffectiveSidebarLinkDefaults())
}

func resyncSidebarLinkDefaultsForAllUsers(doc usersidebar.SidebarLinkDefaultsDocument) error {
	if errInjectResyncSidebarDefaults != nil {
		return errInjectResyncSidebarDefaults
	}
	usersMux.Lock()
	defer usersMux.Unlock()

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for sidebar defaults sync: %w", err)
	}

	var updated int
	for _, row := range usersList {
		if row == nil {
			continue
		}
		u := cloneUserPtr(row)
		links, changed := usersidebar.MergeDefaultLinks(u.SidebarLinks, u.BackendScopes, doc)
		if !changed {
			continue
		}
		u.SidebarLinks = links
		if normalized, normChanged := usersidebar.NormalizeSidebarLinks(u.SidebarLinks); normChanged {
			u.SidebarLinks = normalized
		}
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync sidebar defaults for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced sidebar link defaults for %d users", updated)
	}
	return nil
}

// ResyncEnforcedSidebarLinksForAllUsers merges enforced sidebar links for non-admin users.
func ResyncEnforcedSidebarLinksForAllUsers() error {
	return resyncEnforcedSidebarLinksForAllUsers(EffectiveSidebarLinkDefaults())
}

func resyncEnforcedSidebarLinksForAllUsers(doc usersidebar.SidebarLinkDefaultsDocument) error {
	usersMux.Lock()
	defer usersMux.Unlock()

	usersList, err := sqlDb.ListUsers()
	if err != nil {
		return fmt.Errorf("list users for enforced sidebar sync: %w", err)
	}

	var updated int
	for _, row := range usersList {
		if row == nil {
			continue
		}
		u := cloneUserPtr(row)
		if u.Permissions.Admin {
			continue
		}
		links, changed := usersidebar.MergeEnforcedLinks(u.SidebarLinks, u.BackendScopes, doc, u.Permissions.Admin)
		if !changed {
			continue
		}
		u.SidebarLinks = links
		if normalized, normChanged := usersidebar.NormalizeSidebarLinks(u.SidebarLinks); normChanged {
			u.SidebarLinks = normalized
		}
		u.FrontendScopes = nil
		u.SourcePermissions = nil
		if err := sqlDb.UpdateUser(u); err != nil {
			return fmt.Errorf("sync enforced sidebar links for user %s: %w", u.Username, err)
		}
		putUserInCache(u)
		updated++
	}
	if updated > 0 {
		logger.Debugf("synced enforced sidebar links for %d users", updated)
	}
	return nil
}
