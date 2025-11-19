package cmd

import (
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

// validateShareInfo migrates share links to add default sidebar links
func validateShareInfo() {
	if store.Share == nil {
		return
	}

	// Get all shares
	shares, err := store.Share.All()
	if err != nil {
		logger.Debugf("No shares found or error getting shares: %v", err)
		return
	}

	migratedCount := 0
	for _, link := range shares {
		// Check if this share needs migration (version not set or 0)
		if link.Version == 0 {
			// Add default sidebar links
			link.SidebarLinks = []users.SidebarLink{
				{
					Name:     "Share QR Code and Info",
					Category: "shareInfo",
					Target:   "#",
					Icon:     "qr_code",
				},
				{
					Name:     "Download",
					Category: "download",
					Target:   "#",
					Icon:     "download",
				},
			}

			// Set version to 1 to indicate migration is complete
			link.Version = 1

			// Save the updated share
			if err := store.Share.Save(link); err != nil {
				logger.Errorf("Failed to migrate share %s: %v", link.Hash, err)
				continue
			}

			migratedCount++
		}
	}

	if migratedCount > 0 {
		logger.Infof("Migrated %d share links with default sidebar links", migratedCount)
	}
}
