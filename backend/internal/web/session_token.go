package web

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
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
	if err := state.RegisterBearerToken(tokenString, user.ID); err != nil {
		return "", err
	}
	return tokenString, nil
}

func replaceSessionToken(oldToken string, user *users.User) (string, error) {
	if oldToken != "" {
		_ = state.RevokeToken(oldToken)
		_ = state.RemoveApiToken(oldToken)
	}
	return mintAndRegisterSessionToken(user)
}
