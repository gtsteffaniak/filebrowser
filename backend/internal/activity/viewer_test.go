package activity

import (
	"testing"

	activitydb "github.com/gtsteffaniak/filebrowser/backend/internal/database/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestPrepareItemsForViewerTrimsNonAdminPaths(t *testing.T) {
	viewer := &users.User{
		ID: 2,
		FrontendUser: users.FrontendUser{
			Permissions: users.Permissions{Admin: false},
		},
		BackendScopes: []users.BackendScope{
			{Path: "/source", Scope: "/source/users/graham"},
		},
	}
	items := []activitydb.FrontendEntry{{
		Source: "/source",
		Path:   "/source/users/graham/path/to/file",
		Details: activitydb.FrontendDetails{
			Source: "/source",
			Paths:  []string{"/source/users/graham/path/to/file"},
		},
	}}

	PrepareItemsForViewer(items, viewer)

	if items[0].Path != "/path/to/file" {
		t.Fatalf("Path = %q, want /path/to/file", items[0].Path)
	}
	if items[0].Details.Paths[0] != "/path/to/file" {
		t.Fatalf("Details.Paths[0] = %q, want /path/to/file", items[0].Details.Paths[0])
	}
}

func TestPrepareItemsForViewerSkipsAdmin(t *testing.T) {
	admin := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{
			Permissions: users.Permissions{Admin: true},
		},
	}
	items := []activitydb.FrontendEntry{{
		Source: "/source",
		Path:   "/source/users/graham/path/to/file",
	}}
	PrepareItemsForViewer(items, admin)
	if items[0].Path != "/source/users/graham/path/to/file" {
		t.Fatalf("admin path should remain untrimmed, got %q", items[0].Path)
	}
}
