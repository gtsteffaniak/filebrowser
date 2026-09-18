package settings_test

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMergeDefaultEnabledBackendScopes_RespectsDeclinedSources(t *testing.T) {
	settings.Config.Server.Sources = []*settings.Source{
		{Path: "/pathA", Name: "a", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/a"}},
		{Path: "/pathB", Name: "b", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/b"}},
	}
	t.Cleanup(func() { settings.Config.Server.Sources = nil })

	existing := []users.BackendScope{{Path: "/pathA", Scope: "/a"}}
	got := settings.MergeDefaultEnabledBackendScopes(existing, []string{"/pathB"})
	if len(got) != 1 || got[0].Path != "/pathA" {
		t.Fatalf("declined source re-added: %#v", got)
	}
}

func TestComputeDeclinedDefaultSources(t *testing.T) {
	settings.Config.Server.Sources = []*settings.Source{
		{Path: "/pathA", Name: "a", Config: settings.SourceConfig{DefaultEnabled: true}},
		{Path: "/pathB", Name: "b", Config: settings.SourceConfig{DefaultEnabled: true}},
	}
	t.Cleanup(func() { settings.Config.Server.Sources = nil })

	declined := settings.ComputeDeclinedDefaultSources([]users.BackendScope{{Path: "/pathA", Scope: "/"}})
	if len(declined) != 1 || declined[0] != "/pathB" {
		t.Fatalf("ComputeDeclinedDefaultSources() = %#v", declined)
	}
}
