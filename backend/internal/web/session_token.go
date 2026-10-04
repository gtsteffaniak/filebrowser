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

	tokenString, _, err := auth.MakeSignedTokenAPI(
		user,
		name,
		expires,
		user.Permissions,
		true,
	)
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
// The previous session token is retired only after the replacement token has
// been successfully created and registered. Retired tokens remain usable
// during state.BearerTokenGrace so in-flight requests carrying the old cookie
// are not rejected immediately.
func replaceSessionToken(oldToken string, user *users.User) (string, error) {
	if user == nil || user.ID == 0 {
		return "", fmt.Errorf("invalid user for session token")
	}

	logger.Debugf(
		"AUTH DEBUG: ROTATE start oldHash=%s userID=%d username=%s",
		tokenHashPrefix(oldToken),
		user.ID,
		user.Username,
	)

	// First create and register the new token. This ordering is important:
	// if minting or registration fails, the old token must remain usable.
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

	// Keep the old token alive for the configured grace period so requests
	// already in flight, or clients that have not yet received the new cookie,
	// can still complete successfully.
	if oldToken != "" && oldToken != newToken {
		if err := state.RetireSessionToken(oldToken); err != nil {
			logger.Errorf(
				"AUTH DEBUG: ROTATE retire FAILED oldHash=%s newHash=%s err=%v",
				tokenHashPrefix(oldToken),
				tokenHashPrefix(newToken),
				err,
			)

			// The new token has already been registered. Do not revoke it here:
			// the old token is still usable and the new token remains a valid
			// session token for the client that received it.
			return "", err
		}

		logger.Debugf(
			"AUTH DEBUG: ROTATE retired oldHash=%s grace=%s",
			tokenHashPrefix(oldToken),
			state.BearerTokenGrace,
		)
	}

	logger.Debugf(
		"AUTH DEBUG: ROTATE complete oldHash=%s newHash=%s",
		tokenHashPrefix(oldToken),
		tokenHashPrefix(newToken),
	)

	return newToken, nil
}
