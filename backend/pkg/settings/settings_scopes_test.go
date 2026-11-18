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

func TestMergeDefaultEnabledBackendScopes_PresservesMultipleNestedScopes(t *testing.T) {
	settings.Config.Server.Sources = []*settings.Source{
		{Path: "/data", Name: "source1", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/"}},
	}
	t.Cleanup(func() { settings.Config.Server.Sources = nil })

	// User has two scopes on the same source: root (read-only) and nested child (write)
	existing := []users.BackendScope{
		{Path: "/data", Scope: "/", Permissions: users.SourceFilePermissions{View: true}},
		{Path: "/data", Scope: "/khoi5/5A", Permissions: users.SourceFilePermissions{View: true, Modify: true, Delete: true, Create: true}},
	}
	got := settings.MergeDefaultEnabledBackendScopes(existing, nil)

	if len(got) != 2 {
		t.Fatalf("MergeDefaultEnabledBackendScopes() returned %d scopes, want 2; got %#v", len(got), got)
	}

	// Check both scopes are present with correct permissions
	if !(got[0].Path == "/data" && got[0].Scope == "/") {
		t.Errorf("first scope = {Path: %q, Scope: %q}, want {Path: %q, Scope: %q}", got[0].Path, got[0].Scope, "/data", "/")
	}
	if !(got[0].Permissions.View && !got[0].Permissions.Modify) {
		t.Errorf("first scope permissions = {View: %v, Modify: %v}, want {View: true, Modify: false}", got[0].Permissions.View, got[0].Permissions.Modify)
	}

	if !(got[1].Path == "/data" && got[1].Scope == "/khoi5/5A") {
		t.Errorf("second scope = {Path: %q, Scope: %q}, want {Path: %q, Scope: %q}", got[1].Path, got[1].Scope, "/data", "/khoi5/5A")
	}
	if !(got[1].Permissions.View && got[1].Permissions.Modify && got[1].Permissions.Create) {
		t.Errorf("second scope permissions = {View: %v, Modify: %v, Create: %v}, want all true", got[1].Permissions.View, got[1].Permissions.Modify, got[1].Permissions.Create)
	}
}

func TestMergeDefaultEnabledBackendScopes_QuotaSurvives(t *testing.T) {
	settings.Config.Server.Sources = []*settings.Source{
		{Path: "/data", Name: "source1", Config: settings.SourceConfig{DefaultEnabled: true, DefaultUserScope: "/"}},
	}
	t.Cleanup(func() { settings.Config.Server.Sources = nil })

	quotaVal := int64(1000000)
	quota := &users.ScopeQuota{LimitBytes: quotaVal}
	existing := []users.BackendScope{
		{Path: "/data", Scope: "/", Quota: quota},
	}
	got := settings.MergeDefaultEnabledBackendScopes(existing, nil)

	if len(got) != 1 {
		t.Fatalf("MergeDefaultEnabledBackendScopes() returned %d scopes, want 1", len(got))
	}
	if got[0].Quota == nil {
		t.Errorf("Quota = nil, want %#v", quota)
	} else if got[0].Quota.LimitBytes != quota.LimitBytes {
		t.Errorf("Quota.LimitBytes = %v, want %v", got[0].Quota.LimitBytes, quota.LimitBytes)
	}
}
