package web

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func effectiveFilePerms(d *Context, sourceName string) (users.SourceFilePermissions, error) {
	if d == nil || d.User == nil {
		return users.DenyAllSourceFilePermissions(), fmt.Errorf("user context not set")
	}
	if d.Share.Hash != "" {
		return shareFilePerms(d), nil
	}
	return d.User.FilePermsForSourceName(sourceName)
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
