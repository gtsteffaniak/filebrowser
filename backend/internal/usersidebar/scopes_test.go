package usersidebar

import (
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestNormalizeSidebarLinks_preservesCustomSourceNameAndIcon(t *testing.T) {
	testSourceConfig(t)

	in := []users.SidebarLink{
		{
			Name:       "My Files",
			Category:   string(users.SidebarLinkSourceMinimal),
			Icon:       "folder",
			SourceName: "../frontend/tests/playwright-files",
			Target:     "/",
		},
	}

	out, changed := NormalizeSidebarLinks(in)
	if changed {
		t.Fatal("expected canonical link unchanged")
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	if out[0].Name != "My Files" {
		t.Fatalf("Name = %q, want custom name preserved", out[0].Name)
	}
	if out[0].Icon != "folder" {
		t.Fatalf("Icon = %q, want folder", out[0].Icon)
	}
	if out[0].Category != string(users.SidebarLinkSourceMinimal) {
		t.Fatalf("Category = %q", out[0].Category)
	}
}

func TestEnsureSidebarLinksFromScopes_addsMissingScopedSources(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{
			Name:       "docker",
			Category:   string(users.SidebarLinkSource),
			SourceName: ".",
			Target:     "/",
		},
		{
			Name:     "Docs",
			Category: string(users.SidebarLinkCustom),
			Target:   "https://example.com",
			Icon:     "link",
		},
	}
	scopes := []users.BackendScope{
		{Path: "."},
		{Path: "../frontend/tests/playwright-files"},
	}

	out, changed := EnsureSidebarLinksFromScopes(links, scopes)
	if !changed {
		t.Fatal("expected changed=true")
	}
	if len(out) != 3 {
		t.Fatalf("len(out) = %d, want 3", len(out))
	}
	if out[2].Name != "playwright + files" {
		t.Fatalf("added link = %#v", out[2])
	}
	if out[1].Category != string(users.SidebarLinkCustom) {
		t.Fatalf("custom link lost: %#v", out[1])
	}
}

func TestNeedsSidebarLinksFromScopes_partialCoverage(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
	}
	scopes := []users.BackendScope{
		{Path: "."},
		{Path: "../frontend/tests/playwright-files"},
	}
	if !NeedsSidebarLinksFromScopes(links, scopes) {
		t.Fatal("expected partial scope coverage to need merge")
	}
}

func TestEnsureSidebarLinksFromScopes_idempotent(t *testing.T) {
	testSourceConfig(t)

	links := []users.SidebarLink{
		{Name: "playwright + files", Category: "source", SourceName: "../frontend/tests/playwright-files", Target: "/"},
		{Name: "docker", Category: "source", SourceName: ".", Target: "/"},
	}
	scopes := []users.BackendScope{
		{Path: "../frontend/tests/playwright-files"},
		{Path: "."},
	}

	out, changed := EnsureSidebarLinksFromScopes(links, scopes)
	if changed {
		t.Fatal("expected changed=false")
	}
	if !reflect.DeepEqual(out, links) {
		t.Fatalf("got %#v", out)
	}
}
