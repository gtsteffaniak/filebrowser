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
