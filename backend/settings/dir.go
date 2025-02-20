package settings

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/logger"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[^0-9A-Za-z@_\-.]`)
	dashes               = regexp.MustCompile(`[\-]+`)
)

// MakeUserDir makes the user directory according to settings.
func (s *Settings) MakeUserDirs(username, serverRoot string, scopes map[string]string) (map[string]string, error) {
	userScopes := map[string]string{}
	for key, userScope := range scopes {
		userScope = strings.TrimSpace(userScope)
		if userScope == "" && s.Server.CreateUserDir {
			username = cleanUsername(username)
			if username == "" || username == "-" || username == "." {
				logger.Error(fmt.Sprintf("create user: invalid user for home dir creation: [%s]", username))
			}
			userScope = path.Join(s.Server.UserHomeBasePath, username)
		}

		userScope = path.Join("/", userScope)
		fullPath := filepath.Join(serverRoot, userScope)
		if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
			logger.Error(fmt.Sprintf("failed to create user home dir: [%s]: %v", userScope, err))
		}
		userScopes[key] = userScope

	}

	return userScopes, nil
}

func cleanUsername(s string) string {
	// Remove any trailing space to avoid ending on -
	s = strings.Trim(s, " ")
	s = strings.Replace(s, "..", "", -1)

	// Replace all characters which not in the list `0-9A-Za-z@_\-.` with a dash
	s = invalidFilenameChars.ReplaceAllString(s, "-")

	// Remove any multiple dashes caused by replacements above
	s = dashes.ReplaceAllString(s, "-")
	return s
}
