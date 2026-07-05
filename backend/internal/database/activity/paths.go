package activity

import (
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// TrimPathForUserScope removes a user's index scope prefix from a stored path.
// If the path is outside the scope prefix it is returned unchanged.
func TrimPathForUserScope(path, userScope string) string {
	if userScope == "/" || userScope == "" {
		return path
	}

	path = strings.TrimSpace(path)
	scope := strings.TrimRight(strings.TrimSpace(userScope), "/")
	if path == "" || scope == "" || scope == "/" {
		return path
	}
	if !strings.HasPrefix(scope, "/") {
		scope = "/" + scope
	}
	rest, ok := strings.CutPrefix(path, scope)
	if !ok {
		return path
	}
	if rest == "" {
		return "/"
	}
	return "/" + strings.TrimPrefix(rest, "/")
}

func scopeForUserSource(user *users.User, source string) (string, bool) {
	if user == nil || source == "" {
		return "", false
	}
	if scope, err := user.GetScopeForSourceName(source); err == nil {
		return scope, true
	}
	if scope, err := user.GetScopeForSourcePath(source); err == nil {
		return scope, true
	}
	return "", false
}

func trimPathForUser(user *users.User, source, path string) string {
	if path == "" {
		return path
	}
	if source == "" {
		return path
	}
	scope, ok := scopeForUserSource(user, source)
	if !ok {
		return path
	}
	return TrimPathForUserScope(path, scope)
}

// TrimPathsForUser strips the user's per-source scope prefix from file paths in the API entry.
// User scope settings in Details.Scopes are left unchanged.
func (fe *FrontendEntry) TrimPathsForUser(user *users.User) {
	if user == nil || user.ID == 0 {
		return
	}

	source := fe.Source
	if source == "" {
		source = fe.Details.Source
	}

	if fe.Path != "" {
		fe.Path = trimPathForUser(user, source, fe.Path)
	}
	if fe.TargetPath != "" {
		fe.TargetPath = trimPathForUser(user, source, fe.TargetPath)
	}

	if fe.Details.Path != "" {
		detailsSource := fe.Details.Source
		if detailsSource == "" {
			detailsSource = source
		}
		fe.Details.Path = trimPathForUser(user, detailsSource, fe.Details.Path)
	}
	if fe.Details.TargetPath != "" {
		detailsSource := fe.Details.Source
		if detailsSource == "" {
			detailsSource = source
		}
		fe.Details.TargetPath = trimPathForUser(user, detailsSource, fe.Details.TargetPath)
	}
	if len(fe.Details.Paths) > 0 {
		detailsSource := fe.Details.Source
		if detailsSource == "" {
			detailsSource = source
		}
		for i, p := range fe.Details.Paths {
			fe.Details.Paths[i] = trimPathForUser(user, detailsSource, p)
		}
	}
}
