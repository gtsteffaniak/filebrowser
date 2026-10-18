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
// For shares, owner is the user whose current source permissions are intersected with the share flags.
func EffectiveFilePermissions(user *users.User, link *Share, sourceName string) (users.SourceFilePermissions, error) {
	if link != nil && link.Hash != "" {
		owner := user
		if owner == nil || owner.Username == "" || owner.Username == "anonymous" {
			owner = nil
		}
		sharePerms := link.FilePermissions()
		if owner == nil {
			return sharePerms, nil
		}
		ownerPerms, err := owner.FilePermsForSourceName(sourceName)
		if err != nil {
			return users.DenyAllSourceFilePermissions(), err
		}
		return users.SourceFilePermissions{
			View:     sharePerms.View && ownerPerms.View,
			Download: sharePerms.Download && ownerPerms.Download,
			Modify:   sharePerms.Modify && ownerPerms.Modify,
			Delete:   sharePerms.Delete && ownerPerms.Delete,
			Create:   sharePerms.Create && ownerPerms.Create,
		}, nil
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
