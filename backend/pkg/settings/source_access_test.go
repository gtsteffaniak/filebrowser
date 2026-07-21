package settings_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestBuiltinDefaultSourceFilePermissions(t *testing.T) {
	p := settings.BuiltinDefaultSourceFilePermissions()
	if !p.View || !p.Download || p.Modify || p.Create || p.Delete {
		t.Fatalf("unexpected builtin defaults: %+v", p)
	}
}

func TestNormalizeSourceFilePermissions_unsetUsesBuiltin(t *testing.T) {
	got := settings.NormalizeSourceFilePermissions(users.SourceFilePermissions{})
	if got != settings.BuiltinDefaultSourceFilePermissions() {
		t.Fatalf("got %+v", got)
	}
}

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

func TestSourceFilePermissionsFromLegacyUserDefaults(t *testing.T) {
	d := settings.UserDefaults{}
	d.Permissions.Modify = true
	d.Permissions.Create = true
	d.Permissions.Share = true
	got := settings.SourceFilePermissionsFromLegacyUserDefaults(d)
	if !got.View || !got.Download || !got.Modify || !got.Create || !got.Configured {
		t.Fatalf("legacy map: %+v", got)
	}
}

func TestApplySourceAccessDefaultsToAllSources(t *testing.T) {
	t.Setenv("FILEBROWSER_ONLYOFFICE_SECRET", "")
	settings.Initialize("")
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
		if !src.Config.DefaultPermissions.Configured {
			t.Fatal("expected configured on applied defaults")
		}
	}
}

func TestDefaultSourceFilePermissions_fromSourceConfig(t *testing.T) {
	settings.Config.Server.Sources = []*settings.Source{
		{
			Path: "/only",
			Name: "only",
			Config: settings.SourceConfig{
				DefaultPermissions: users.SourceFilePermissions{
					View: true, Download: false, Configured: true,
				},
			},
		},
	}
	got := settings.DefaultSourceFilePermissions()
	if !got.View || got.Download {
		t.Fatalf("got %+v", got)
	}
}
