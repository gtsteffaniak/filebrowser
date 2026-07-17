package users

import "testing"

func TestSanitizeTokenPermissionsStripsFileOps(t *testing.T) {
	got := SanitizeTokenPermissions(Permissions{
		Admin:    true,
		Api:      true,
		Modify:   true,
		View:     true,
		Download: true,
	})
	if got.Modify || got.View || got.Download || got.Create || got.Delete {
		t.Fatalf("file ops should be stripped, got %#v", got)
	}
	if !got.Admin || !got.Api {
		t.Fatalf("globals should remain, got %#v", got)
	}
}

func TestIntersectGlobalPermissions(t *testing.T) {
	owner := Permissions{Admin: true, Api: true, Share: true, Realtime: false}
	caps := Permissions{Admin: true, Api: false, Share: true, Realtime: true, Modify: true}
	got := IntersectGlobalPermissions(owner, caps)
	want := Permissions{Admin: true, Api: false, Share: true, Realtime: false}
	if got != want {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestHasAnyGlobalPermission(t *testing.T) {
	if HasAnyGlobalPermission(Permissions{Modify: true}) {
		t.Fatal("file ops alone should not count as global")
	}
	if !HasAnyGlobalPermission(Permissions{Api: true}) {
		t.Fatal("expected api to count as global")
	}
}

func TestIsMinimalApiToken(t *testing.T) {
	if !IsMinimalApiToken(AuthToken{}) {
		t.Fatal("empty token should be minimal")
	}
	if IsMinimalApiToken(AuthToken{BelongsTo: 1, Permissions: Permissions{Admin: true}}) {
		t.Fatal("custom token with global caps should not be minimal")
	}
	if !IsMinimalApiToken(AuthToken{BelongsTo: 1, Permissions: Permissions{Modify: true}}) {
		t.Fatal("token without global caps should be minimal")
	}
}
