package share

import "github.com/gtsteffaniak/filebrowser/backend/database/users"

// ClampShareEditable limits client-requested share flags to what the owner is allowed.
func ClampShareEditable(ownerPerms users.Permissions, cs *CommonShare) {
	if cs == nil {
		return
	}
	if !ownerPerms.Modify {
		cs.AllowModify = false
	}
	if !ownerPerms.Create {
		cs.AllowCreate = false
	}
	if !ownerPerms.Delete {
		cs.AllowDelete = false
	}
	if !ownerPerms.Download {
		cs.DisableDownload = true
	}
}
