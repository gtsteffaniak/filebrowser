package users

import (
	"encoding/json"
	"testing"
)

func TestPermissionsUnmarshalJSONMarksConfigured(t *testing.T) {
	t.Parallel()

	var p Permissions
	if err := json.Unmarshal([]byte(`{"admin":false,"share":false,"api":false,"realtime":false}`), &p); err != nil {
		t.Fatal(err)
	}
	if !p.Configured {
		t.Fatal("expected Configured after unmarshaling an explicit permissions object")
	}
	if p.Admin || p.Share || p.Api || p.Realtime {
		t.Fatalf("expected all-false permissions, got %+v", p)
	}

	var u struct {
		Permissions Permissions `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(`{}`), &u); err != nil {
		t.Fatal(err)
	}
	if u.Permissions.Configured {
		t.Fatal("omitted permissions must not set Configured")
	}

	var nullWrap struct {
		Permissions Permissions `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(`{"permissions":null}`), &nullWrap); err != nil {
		t.Fatal(err)
	}
	if nullWrap.Permissions.Configured {
		t.Fatal("permissions null must not set Configured")
	}
	if nullWrap.Permissions.GlobalPermissionsFieldMaskFromPermissions().Any() {
		t.Fatal("permissions null must not set global key mask")
	}
}

func TestPermissionsUnmarshalJSONPartialKeys(t *testing.T) {
	t.Parallel()

	var u struct {
		Permissions Permissions `json:"permissions"`
	}
	if err := json.Unmarshal([]byte(`{"permissions":{"admin":true}}`), &u); err != nil {
		t.Fatal(err)
	}
	if !u.Permissions.Configured {
		t.Fatal("expected Configured for explicit permissions object")
	}
	mask := u.Permissions.GlobalPermissionsFieldMaskFromPermissions()
	if !mask.Admin || mask.Api || mask.Share || mask.Realtime {
		t.Fatalf("mask = %+v, want admin only", mask)
	}
	if !u.Permissions.Admin {
		t.Fatal("expected admin true")
	}
}

func TestSourceFilePermissionsIsUnset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		perms  SourceFilePermissions
		unset  bool
	}{
		{name: "all false", perms: SourceFilePermissions{}, unset: true},
		{name: "configured deny-all", perms: DenyAllSourceFilePermissions(), unset: false},
		{name: "view only", perms: SourceFilePermissions{View: true}, unset: false},
		{name: "download only", perms: SourceFilePermissions{Download: true}, unset: false},
		{name: "full", perms: SourceFilePermissions{View: true, Download: true, Modify: true, Create: true, Delete: true}, unset: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.perms.IsUnset(); got != tt.unset {
				t.Fatalf("IsUnset() = %v, want %v", got, tt.unset)
			}
		})
	}
}

func TestPermissionsHasAnyFilePermission(t *testing.T) {
	t.Parallel()

	if (Permissions{}).HasAnyFilePermission() {
		t.Fatal("expected empty permissions to have no file caps")
	}
	if !(Permissions{View: true}).HasAnyFilePermission() {
		t.Fatal("expected view to count as file cap")
	}
}
