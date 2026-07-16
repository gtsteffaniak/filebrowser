package web

import (
	"fmt"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func userHasAnySourcePerm(u *users.User, check func(users.SourceFilePermissions) bool) bool {
	if u == nil || u.BackendSourcePermissions == nil {
		return false
	}
	for _, perms := range u.BackendSourcePermissions {
		if check(perms) {
			return true
		}
	}
	return false
}

func effectiveFilePerms(d *Context, sourceName string) (users.SourceFilePermissions, error) {
	if d == nil || d.User == nil {
		return users.DenyAllSourceFilePermissions(), fmt.Errorf("user context not set")
	}
	if d.Share.Hash != "" {
		return shareFilePerms(d), nil
	}
	perms, err := d.User.FilePermsForSourceName(sourceName)
	if err != nil {
		return users.DenyAllSourceFilePermissions(), err
	}
	if tokenPerms, ok := apiTokenSourceFilePerms(d); ok {
		perms = users.IntersectSourceFilePermissions(perms, tokenPerms)
	}
	return perms, nil
}

func shareFilePerms(d *Context) users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     !d.Share.DisableFileViewer,
		Download: !d.Share.DisableDownload,
		Modify:   d.Share.AllowModify,
		Delete:   d.Share.AllowDelete,
		Create:   d.Share.AllowCreate,
	}
}

func apiTokenSourceFilePerms(d *Context) (users.SourceFilePermissions, bool) {
	if d.Token == "" {
		return users.SourceFilePermissions{}, false
	}
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(settings.Config.Auth.Key), nil
	}
	var tk users.AuthToken
	token, err := jwt.ParseWithClaims(d.Token, &tk, keyFunc)
	if err != nil || !token.Valid {
		return users.SourceFilePermissions{}, false
	}
	// Session WEB_TOKEN claims carry global perms only after v4; intersect only when token sets file-op caps.
	hasFileCaps := tk.Permissions.View || tk.Permissions.Download || tk.Permissions.Modify ||
		tk.Permissions.Delete || tk.Permissions.Create
	if !hasFileCaps {
		return users.SourceFilePermissions{}, false
	}
	return permissionsFromTokenClaims(tk), true
}

func permissionsFromTokenClaims(tk users.AuthToken) users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     tk.Permissions.View,
		Download: tk.Permissions.Download,
		Modify:   tk.Permissions.Modify,
		Delete:   tk.Permissions.Delete,
		Create:   tk.Permissions.Create,
	}
}
