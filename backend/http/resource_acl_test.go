package http

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

func TestResourcePostACLUsesFullIndexPathKey(t *testing.T) {
	setupTestEnv(t)

	const (
		sourcePath   = "/srv"
		userScope    = "/home/alice"
		deniedFolder = "/home/alice/projects/acme/private"
		relPath      = "/projects/acme/private/poison.docx"
	)
	fullIndexPath := utils.JoinPathAsUnix(userScope, relPath)

	alice := &users.User{
		ID:       1,
		Username: "alice",
	}
	if err := store.Users.Save(alice, true, true); err != nil {
		t.Fatal(err)
	}

	if err := store.Access.DenyUser(sourcePath, deniedFolder, "alice"); err != nil {
		t.Fatalf("DenyUser: %v", err)
	}

	if !store.Access.Permitted(sourcePath, relPath, "alice") {
		t.Fatal("scope-stripped path does not match deny rule keyed at full index path (documents pre-fix ACL miss)")
	}
	if store.Access.Permitted(sourcePath, fullIndexPath, "alice") {
		t.Fatalf("full index path %q should be denied for alice", fullIndexPath)
	}
}
