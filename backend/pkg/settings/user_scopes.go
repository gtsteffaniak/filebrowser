package settings

import (
	"path"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// ExpandBackendScopesForCreateUserDir appends the username to each default scope when the
// source has createUserDir enabled. Must run before persisting a new user so JWT/proxy/OIDC
// auto-create stores the personal folder scope, not only the source root.
func ExpandBackendScopesForCreateUserDir(u *users.User) {
	if u == nil || u.Username == "" || u.Username == users.AnonymousUserName {
		return
	}
	cleanedUserName := users.CleanUsername(u.Username)
	if cleanedUserName == "" || cleanedUserName == "-" || cleanedUserName == "." {
		return
	}
	for i, scope := range u.BackendScopes {
		source, ok := Config.Server.SourceMap[scope.Path]
		if !ok {
			continue
		}
		if !source.Config.CreateUserDir {
			continue
		}
		// This runs more than once for a new user, so a scope that already ends in the user's
		// directory is left alone. The source's defaultUserScope itself is not a user's home:
		// a user named after its last element ("users" with "/users") gets "/users/users".
		if scopeBaseName(scope.Scope) == cleanedUserName && !sameScopePath(scope.Scope, source.Config.DefaultUserScope) {
			continue
		}
		u.BackendScopes[i].Scope = joinUnixScopePath(scope.Scope, cleanedUserName)
	}
}

func sameScopePath(a, b string) bool {
	return path.Clean("/"+a) == path.Clean("/"+b)
}

func scopeBaseName(scope string) string {
	scope = strings.TrimSuffix(scope, "/")
	if scope == "" || scope == "/" {
		return "/"
	}
	i := strings.LastIndex(scope, "/")
	if i < 0 {
		return scope
	}
	return scope[i+1:]
}

func joinUnixScopePath(base, elem string) string {
	base = strings.TrimSuffix(base, "/")
	if base == "" || base == "/" {
		return "/" + strings.TrimPrefix(elem, "/")
	}
	return base + "/" + strings.TrimPrefix(elem, "/")
}
