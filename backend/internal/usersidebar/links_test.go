package usersidebar

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestFrontendLinks_populatesEmptySourceName(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{Category: "source", SourceName: ".", Target: "/"},
		{Name: "My Vault", Category: string(users.SidebarLinkSourceMinimal), Icon: "folder", SourceName: "../frontend/tests/playwright-files", Target: "/docs"},
	}

	out := FrontendLinks(in, false)
	if len(out) != 2 {
		t.Fatalf("len(out) = %d, want 2", len(out))
	}
	if out[0].Name != "docker" || out[0].SourceName != "docker" {
		t.Fatalf("default source link = %#v", out[0])
	}
	if out[1].Name != "My Vault" || out[1].SourceName != "playwright + files" {
		t.Fatalf("custom source link = %#v", out[1])
	}
}
