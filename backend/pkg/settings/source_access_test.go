package settings_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestNormalizeSourceFilePermissions_explicitDenyAllPreserved(t *testing.T) {
	deny := users.DenyAllSourceFilePermissions()
	got := settings.NormalizeSourceFilePermissions(deny)
	if got.View || got.Download || got.Modify || got.Create || got.Delete {
		t.Fatalf("deny-all should stay false: %+v", got)
	}
	if !got.Configured {
		t.Fatal("expected configured marker on deny-all")
	}
}

func TestApplySourceAccessDefaultsToAllSources(t *testing.T) {
	settings.Config = settings.SetDefaults(true)
	settings.Config.Server.Sources = []*settings.Source{
		{Path: "/a", Name: "a"},
		{Path: "/b", Name: "b"},
	}
	perms := users.SourceFilePermissions{View: true, Modify: true, Configured: true}
	settings.ApplySourceAccessDefaultsToAllSources(perms)
	for _, src := range settings.Config.Server.Sources {
		if !src.Config.DefaultPermissions.View || !src.Config.DefaultPermissions.Modify {
			t.Fatalf("source %s: %+v", src.Name, src.Config.DefaultPermissions)
		}
	}
}
