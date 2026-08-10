package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestBackendScopesForGroups_ClaimValue(t *testing.T) {
	prev := Config.Server.Sources
	prevMap := Config.Server.SourceMap
	t.Cleanup(func() {
		Config.Server.Sources = prev
		Config.Server.SourceMap = prevMap
	})

	Config.Server.Sources = []*Source{
		{Path: "/data/public", Name: "public", Config: SourceConfig{DefaultEnabled: true, DefaultUserScope: "/"}},
		{Path: "/data/acme", Name: "acme", Config: SourceConfig{DefaultEnabled: true, DefaultUserScope: "/", ClaimValue: "acme"}},
		{Path: "/data/globex", Name: "globex", Config: SourceConfig{DefaultEnabled: true, DefaultUserScope: "/", ClaimValue: "globex"}},
		{Path: "/data/disabled", Name: "off", Config: SourceConfig{DefaultEnabled: false, ClaimValue: "acme"}},
	}
	Config.Server.SourceMap = map[string]*Source{}
	for _, s := range Config.Server.Sources {
		Config.Server.SourceMap[s.Path] = s
	}

	got := BackendScopesForGroups(nil)
	if len(got) != 1 || got[0].Path != "/data/public" {
		t.Fatalf("nil groups should only get public sources, got %#v", got)
	}

	got = BackendScopesForGroups([]string{"acme"})
	paths := map[string]bool{}
	for _, s := range got {
		paths[s.Path] = true
	}
	if !paths["/data/public"] || !paths["/data/acme"] || paths["/data/globex"] || paths["/data/disabled"] {
		t.Fatalf("acme groups scopes: %#v", got)
	}

	u := &users.User{}
	ApplyUserDefaultsFrom(u, Config.UserDefaults)
	if len(u.BackendScopes) != 1 || u.BackendScopes[0].Path != "/data/public" {
		t.Fatalf("ApplyUserDefaults should only grant public sources, got %#v", u.BackendScopes)
	}
}
