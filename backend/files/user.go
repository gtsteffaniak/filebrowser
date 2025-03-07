package files

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/logger"
	"github.com/gtsteffaniak/filebrowser/backend/settings"
	"github.com/gtsteffaniak/filebrowser/backend/users"
)

var (
	invalidFilenameChars = regexp.MustCompile(`[^0-9A-Za-z@_\-.]`)
	dashes               = regexp.MustCompile(`[\-]+`)
)

// todo fix this!!
// MakeUserDir makes the user directory according to settings.
func (idx *Index) MakeUserDir(username string, scope string) error {
	if idx.Config.CreateUserDir {
		username = cleanUsername(username)
		if username == "" || username == "-" || username == "." {
			logger.Error(fmt.Sprintf("create user: invalid user for home dir creation: [%s]", username))
		}
	}
	userScope := path.Join("/", scope)
	fullPath := filepath.Join(idx.Path, userScope)
	logger.Debug(fmt.Sprintf("creating user home dir: [%s]", fullPath))
	if err := os.MkdirAll(fullPath, os.ModePerm); err != nil {
		return err
	}
	return nil
}

func MakeUserDirs(u *users.User) {
	for _, scope := range u.Scopes {
		source := settings.Config.Server.NameToSource[scope.Name]
		if source.Config.CreateUserDir {
			idx := GetIndex(source.Name)
			if idx == nil {
				logger.Error(fmt.Sprintf("create user: failed to find source index for user home dir creation: %s", source.Name))
				continue
			}
			err := idx.MakeUserDir(u.Username, scope.Scope)
			if err != nil {
				logger.Error(fmt.Sprintf("create user: failed to create user home dir: %s", err))
			}
		}
	}
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
