package cmd

import (
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestUpdateSidebarLinks_dedupesDuplicateMigratedLinks(t *testing.T) {
	settingsConfig := "../../_docker/src/settings/backend/config.yaml"
	settings.Initialize(settingsConfig)

	user := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "admin",
			NonAdminEditable: users.NonAdminEditable{
				SidebarLinks: []users.SidebarLink{
					{Name: "playwright + files", Category: "source", SourceName: "/app/frontend/tests/playwright-files", Target: "/"},
					{Name: "docker", Category: "source", SourceName: "/app/backend", Target: "/"},
					{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
					{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
					{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
					{Name: "access", Category: "source", SourceName: "/tests/playwright-files", Target: "/"},
				},
			},
		},
		BackendScopes: []users.BackendScope{
			{Path: "../frontend/tests/playwright-files", Scope: "/"},
			{Path: ".", Scope: "/"},
			{Path: "/tests/playwright-files", Scope: "/"},
		},
	}

	if !updateSidebarLinks(user) {
		t.Fatal("expected updateSidebarLinks to return true")
	}

	count := 0
	for _, link := range user.SidebarLinks {
		if strings.HasPrefix(link.Category, "source") {
			count++
		}
	}
	if count != 3 {
		t.Fatalf("got %d source links, want 3: %v", count, user.SidebarLinks)
	}
}
