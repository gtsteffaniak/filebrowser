package web

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// mintAndRegisterSessionToken creates a minimal session JWT and registers its hash for auth lookup.
func mintAndRegisterSessionToken(user *users.User) (string, error) {
	if user == nil || user.ID == 0 {
		return "", fmt.Errorf("invalid user for session token")
	}

	expires := time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)
	name := "WEB_TOKEN_" + utils.InsecureRandomIdentifier(4)

	tokenString, _, err := auth.MakeSignedTokenAPI(user, name, expires, user.Permissions, true)
	if err != nil {
		return "", err
	}

	if err := state.RegisterSessionToken(tokenString, user.ID); err != nil {
		return "", err
	}

	return tokenString, nil
}

func tokenHashPrefix(token string) string {
	if token == "" {
		return "<empty>"
	}

	return utils.HashSHA256(token)[:8]
}

// replaceSessionToken mints and registers a replacement token.
//
// IMPORTANT:
// The previous session token is intentionally NOT retired here.
// This matches the behavior of the older FileBrowser version,
// where renewing the session created a new token without
// immediately invalidating the previous token.
func replaceSessionToken(oldToken string, user *users.User) (string, error) {
	logger.Debugf(
		"AUTH DEBUG: ROTATE start oldHash=%s userID=%d username=%s",
		tokenHashPrefix(oldToken),
		user.ID,
		user.Username,
	)

	newToken, err := mintAndRegisterSessionToken(user)
	if err != nil {
		logger.Errorf(
			"AUTH DEBUG: ROTATE mint FAILED oldHash=%s err=%v",
			tokenHashPrefix(oldToken),
			err,
		)
		return "", err
	}

	logger.Debugf(
		"AUTH DEBUG: ROTATE newHash=%s oldHash=%s",
		tokenHashPrefix(newToken),
		tokenHashPrefix(oldToken),
	)

	logger.Debugf(
		"AUTH DEBUG: ROTATE complete oldHash=%s newHash=%s",
		tokenHashPrefix(oldToken),
		tokenHashPrefix(newToken),
	)

	return newToken, nil
}
