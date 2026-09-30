package state

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

const bootstrapAdminPasswordHexBytes = 6 // 12 hex characters

// bootstrapDefaultAdminPassword returns the plaintext password for initial admin creation.
// When config adminPassword is blank or "admin", a random password is generated and logged once.
func bootstrapDefaultAdminPassword(username string) (plaintext string, generated bool, err error) {
	cfg := settings.Config.Auth.AdminPassword
	if cfg != "" && cfg != "admin" {
		return cfg, false, nil
	}

	plaintext, err = utils.RandomHex(bootstrapAdminPasswordHexBytes)
	if err != nil {
		return "", false, fmt.Errorf("generate bootstrap admin password: %w", err)
	}
	logger.Infof(
		"Generated initial admin password for user %q (set auth.adminPassword in config to override on future resets): %s",
		username,
		plaintext,
	)
	return plaintext, true, nil
}
