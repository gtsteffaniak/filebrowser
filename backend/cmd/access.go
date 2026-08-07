package cmd

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// validateAccessRules migrates old-style access rules (without trailing slashes) to new format
func validateAccessRules() {
	for sourcePath := range settings.Config.Server.SourceMap {
		rules, err := state.GetAllRules(sourcePath)
		if err != nil {
			logger.Errorf("Failed to get access rules for source %s: %v", sourcePath, err)
			continue
		}

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
			if oldPath != "/" && !strings.HasSuffix(oldPath, "/") {
				newPath := oldPath + "/"

				if err := migrateAccessRule(sourcePath, newPath, rule); err != nil {
					logger.Errorf("Failed to migrate rule from %s to %s: %v", oldPath, newPath, err)
					continue
				}

				state.RemoveRuleByPathKey(sourcePath, oldPath)
				migratedCount++
			}
		}

		if migratedCount > 0 {
			logger.Infof("Migrated %d access rules for source %s", migratedCount, sourcePath)
		}
	}
}

func migrateAccessRule(sourcePath, newPath string, rule access.FrontendAccessRule) error {
	for _, user := range rule.Deny.Users {
		if err := state.DenyUser(sourcePath, utils.IndexPathFromNormalized(newPath, true), user); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny user %s: %w", user, err)
		}
	}

	for _, group := range rule.Deny.Groups {
		if err := state.DenyGroup(sourcePath, utils.IndexPathFromNormalized(newPath, true), group); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny group %s: %w", group, err)
		}
	}

	for _, user := range rule.Allow.Users {
		if err := state.AllowUser(sourcePath, utils.IndexPathFromNormalized(newPath, true), user); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add allow user %s: %w", user, err)
		}
	}

	for _, group := range rule.Allow.Groups {
		if err := state.AllowGroup(sourcePath, utils.IndexPathFromNormalized(newPath, true), group); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add allow group %s: %w", group, err)
		}
	}

	if rule.DenyAll {
		if err := state.DenyAll(sourcePath, utils.IndexPathFromNormalized(newPath, true)); err != nil && err != errors.ErrExist {
			return fmt.Errorf("failed to add deny all rule: %w", err)
		}
	}

	return nil
}
