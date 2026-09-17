package usersidebar

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestPruneSidebarLinksForScopes_removesRevokedSourceAndFolderShortcut(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
		{Name: "Photos", Category: string(users.SidebarLinkSourceMinimal), Icon: "photo", SourceName: ".", Target: "/photos"},
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
		{Name: "Docs", Category: string(users.SidebarLinkCustom), Target: "https://example.com", Icon: "link"},
	}
	scopes := []users.BackendScope{
		{Path: "."},
	}

	out, changed := PruneSidebarLinksForScopes(links, scopes)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3 (playwright link removed, docker + folder + custom kept)", len(out))
	}
	if out[2].Category != string(users.SidebarLinkCustom) {
		t.Fatalf("custom link lost: %#v", out[2])
	}
}

func TestPruneSidebarLinksForScopes_noChangeWhenAllScoped(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
	}
	scopes := []users.BackendScope{
		{Path: "."},
	}

	out, changed := PruneSidebarLinksForScopes(links, scopes)
	if changed {
		t.Fatal("expected changed=false")
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
}
