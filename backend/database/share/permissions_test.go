package share

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestClampShareEditable(t *testing.T) {
	t.Parallel()

	ownerPerms := users.Permissions{
		Modify:   false,
		Create:   true,
		Delete:   false,
		Download: false,
	}
	cs := &CommonShare{
		AllowModify:  true,
		AllowCreate:  true,
		AllowDelete:  true,
		DisableDownload: false,
	}

	ClampShareEditable(ownerPerms, cs)

	if cs.AllowModify {
		t.Fatal("expected AllowModify clamped to false")
	}
	if !cs.AllowCreate {
		t.Fatal("expected AllowCreate to remain true")
	}
	if cs.AllowDelete {
		t.Fatal("expected AllowDelete clamped to false")
	}
	if !cs.DisableDownload {
		t.Fatal("expected DisableDownload forced true when owner lacks download")
	}
}
