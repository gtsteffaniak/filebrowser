package cmd

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/database/state"
	"github.com/gtsteffaniak/go-logger/logger"
)

// validateAccessRules migrates old-style access rules (without trailing slashes) to new format
func validateAccessRules() {
	accessStore := state.GetAccessStorage()
	if accessStore == nil {
		return
	}
	// Get all sources
	for sourcePath := range settings.Config.Server.SourceMap {
		// Get all rules for this source
		rules, err := accessStore.GetAllRules(sourcePath)
		if err != nil {
			logger.Errorf("Failed to get access rules for source %s: %v", sourcePath, err)
			continue
		}

		// Check if there are any rules that need migration
		needsMigration := false
		for oldPath := range rules {
			if oldPath != "/" && !strings.HasSuffix(oldPath, "/") {
				needsMigration = true
				break
			}
		}

		if !needsMigration {
			continue
		}

		migratedCount := 0
		for oldPath, rule := range rules {
			// Check if this path needs migration (doesn't have trailing slash and isn't root)
			if oldPath != "/" && !strings.HasSuffix(oldPath, "/") {
				// Create the new path with trailing slash
				newPath := oldPath + "/"

				// Migrate the rule to the new path
				if err := migrateAccessRule(accessStore, sourcePath, oldPath, newPath, rule); err != nil {
					logger.Errorf("Failed to migrate rule from %s to %s: %v", oldPath, newPath, err)
					continue
				}

				// Remove the old rule
				if err := removeOldAccessRule(accessStore, sourcePath, oldPath); err != nil {
					logger.Errorf("Failed to remove old rule %s: %v", oldPath, err)
					continue
				}

				migratedCount++
			}
		}

		// After migration, clear cache
		if migratedCount > 0 {
			logger.Infof("Migrated %d access rules for source %s", migratedCount, sourcePath)
		}
	}
}

// migrateAccessRule creates a new access rule with the new path format
func migrateAccessRule(accessStore *access.Storage, sourcePath, oldPath, newPath string, rule access.FrontendAccessRule) error {
	// Add deny users
	for _, user := range rule.Deny.Users {
		if err := accessStore.DenyUser(sourcePath, newPath, user); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny user %s: %w", user, err)
		}
	}

	// Add deny groups
	for _, group := range rule.Deny.Groups {
		if err := accessStore.DenyGroup(sourcePath, newPath, group); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny group %s: %w", group, err)
		}
	}

	// Add allow users
	for _, user := range rule.Allow.Users {
		if err := accessStore.AllowUser(sourcePath, newPath, user); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add allow user %s: %w", user, err)
		}
	}

	// Add allow groups
	for _, group := range rule.Allow.Groups {
		if err := accessStore.AllowGroup(sourcePath, newPath, group); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add allow group %s: %w", group, err)
		}
	}

	// Add deny all rule if needed
	if rule.DenyAll {
		if err := accessStore.DenyAll(sourcePath, newPath); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny all rule: %w", err)
		}
	}

	return nil
}

// removeOldAccessRule removes the old access rule by directly accessing the internal storage
func removeOldAccessRule(accessStore *access.Storage, sourcePath, oldPath string) error {
	// Access the internal storage directly to remove the old rule
	// We need to use the unnormalized path since that's how it was stored originally
	accessStore.RemoveRuleByPath(sourcePath, oldPath)
	return nil
}
