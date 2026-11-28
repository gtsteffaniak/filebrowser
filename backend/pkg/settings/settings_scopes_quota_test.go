package settings_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestMergeDefaultEnabledBackendScopes_PreservesPersistedScope(t *testing.T) {
	old := settings.Config.Server.Sources
	t.Cleanup(func() { settings.Config.Server.Sources = old })
	for _, enabled := range []bool{true, false} {
		for _, scope := range []string{"/private", ""} {
			for _, meter := range []string{"index_scope", "accounted"} {
				t.Run(fmt.Sprintf("enabled=%t/scope=%q/meter=%s", enabled, scope, meter), func(t *testing.T) {
					settings.Config.Server.Sources = []*settings.Source{
						{Path: "/source", Config: settings.SourceConfig{DefaultEnabled: enabled, DefaultUserScope: "/default"}},
					}
					original := []users.BackendScope{{
						Path: "/source", Scope: scope,
						Permissions: users.SourceFilePermissions{View: true, Create: true, Configured: true},
						Quota:       &users.ScopeQuota{ID: "persisted-quota", LimitBytes: 30 * 1024 * 1024, Meter: meter},
					}}
					// Use separate decoded copies so an in-place mutation cannot change the expected value.
					data, err := json.Marshal(original)
					if err != nil {
						t.Fatal(err)
					}
					var existing, want []users.BackendScope
					if err := json.Unmarshal(data, &existing); err != nil {
						t.Fatal(err)
					}
					if err := json.Unmarshal(data, &want); err != nil {
						t.Fatal(err)
					}
					if scope == "" {
						want[0].Scope = "/default"
					}
					got := settings.MergeDefaultEnabledBackendScopes(existing, nil)
					if len(got) != 1 {
						t.Fatalf("got %d scopes, want 1", len(got))
					}
					if got[0].Quota == nil || got[0].Quota.ID != "persisted-quota" || got[0].Quota.LimitBytes != 30*1024*1024 || got[0].Quota.Meter != meter {
						t.Fatalf("persisted quota changed: got %+v, want %+v", got[0].Quota, want[0].Quota)
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("persisted scope changed: got %#v, want %#v", got, want)
					}
					if !reflect.DeepEqual(existing, original) {
						t.Fatal("merge mutated its input")
					}
					// Compare against the independent expected value after a second merge.
					if again := settings.MergeDefaultEnabledBackendScopes(got, nil); !reflect.DeepEqual(again, want) {
						t.Fatalf("merge is not idempotent: got %#v, want %#v", again, want)
					}
				})
			}
		}
	}
}
