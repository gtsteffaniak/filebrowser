package web

import "github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"

func init() {
	files.RegisterResourceMetadataHooks(files.ResourceMetadataHooks{
		Preflight: preflightDAVProperties,
		Delete:    davDeletePhysicalProperties,
		Move:      davMovePhysicalProperties,
		Copy:      davCopyPhysicalProperties,
	})
}
