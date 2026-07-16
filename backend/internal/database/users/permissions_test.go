package users

import "testing"

func TestSourceFilePermissionsIsUnset(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		perms  SourceFilePermissions
		unset  bool
	}{
		{name: "all false", perms: SourceFilePermissions{}, unset: true},
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
