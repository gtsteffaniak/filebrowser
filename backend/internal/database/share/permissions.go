package share

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// FilePermissions maps share settings to per-source file operation flags.
func (l *Share) FilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     !l.DisableFileViewer,
		Download: !l.DisableDownload,
		Modify:   l.AllowModify,
		Delete:   l.AllowDelete,
		Create:   l.AllowCreate,
	}
}

// EffectiveFilePermissions returns share-scoped permissions when link is active,
// otherwise resolves permissions from the authenticated user.
func EffectiveFilePermissions(user *users.User, link *Share, sourceName string) (users.SourceFilePermissions, error) {
	if link != nil && link.Hash != "" {
		return link.FilePermissions(), nil
	}
	if user == nil {
		return users.DenyAllSourceFilePermissions(), fmt.Errorf("user context not set")
	}
	return user.FilePermsForSourceName(sourceName)
}

// ClampShareEditable limits client-requested share flags to what the owner is allowed on the source.
func ClampShareEditable(ownerPerms users.SourceFilePermissions, editable *ShareEditable) {
	if editable == nil {
		return
	}
	editable.AllowModify = editable.AllowModify && ownerPerms.Modify
	editable.AllowCreate = editable.AllowCreate && ownerPerms.Create
	editable.AllowDelete = editable.AllowDelete && ownerPerms.Delete
	if !ownerPerms.Download {
		editable.DisableDownload = true
	}
	if !ownerPerms.View {
		editable.DisableFileViewer = true
		editable.EnableOnlyOffice = false
	}
}
