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
	if out[2].Name != "" {
		t.Fatalf("added scope link should have empty default name, got %#v", out[2])
	}
}

func TestPrepareSidebarLinksForPersist_prunesLinksWhenScopeRemoved(t *testing.T) {
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
	if !changed {
		t.Fatal("expected changed=true when revoked scope link is pruned")
	}
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2 (revoked scope link removed)", len(out))
	}
}
