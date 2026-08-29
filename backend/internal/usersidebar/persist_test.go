package usersidebar

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestPrepareSidebarLinksForPersist_addsScopedSourcesAndPreservesFolders(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "Photos", Category: string(users.SidebarLinkSourceMinimal), Icon: "photo", SourceName: ".", Target: "/photos"},
	}
	scopes := []users.BackendScope{
		{Path: "."},
		{Path: "../frontend/tests/playwright-files"},
	}

	out, changed := PrepareSidebarLinksForPersist(links, scopes)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if out[1].Name != "Photos" || out[1].Icon != "photo" {
		t.Fatalf("folder shortcut lost: %#v", out[1])
	}
	if out[2].Name != "playwright + files" {
		t.Fatalf("added scope link = %#v", out[2])
	}
}

func TestPrepareSidebarLinksForPersist_keepsLinksWhenScopeRemoved(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "Photos", Category: string(users.SidebarLinkSourceMinimal), Icon: "photo", SourceName: ".", Target: "/photos"},
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
	}
	scopes := []users.BackendScope{
		{Path: "."},
	}

	out, changed := PrepareSidebarLinksForPersist(links, scopes)
	if changed {
		t.Fatal("expected changed=false when only pruning is not performed")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3 (stale scope link kept)", len(out))
	}
}
