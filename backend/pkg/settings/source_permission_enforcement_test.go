package settings

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestSyncEnforcedSourcePermissionsOntoUser_setsCreate(t *testing.T) {
	defaults := users.SourceFilePermissions{
		View: true, Download: true, Modify: false, Create: true, Delete: false, Configured: true,
	}
	enforced := SourceFilePermissionsEnforcement{Create: true}
	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
		BackendScopes: []users.BackendScope{{
			Path:        "/data",
			Permissions: users.SourceFilePermissions{View: true, Download: true, Configured: true},
		}},
	}
	if !SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced) {
		t.Fatal("expected scope permissions to change")
	}
	if !u.BackendScopes[0].Permissions.Create {
		t.Fatal("expected create true on scope after enforced sync")
	}
	if SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced) {
		t.Fatal("expected no further changes when aligned")
	}
}

func TestSyncEnforcedSourcePermissionsOntoUser_skipsAdmin(t *testing.T) {
	defaults := users.SourceFilePermissions{Create: true, Configured: true}
	enforced := SourceFilePermissionsEnforcement{Create: true}
	u := &users.User{
		FrontendUser: users.FrontendUser{
			Username: "admin",
			Permissions: users.Permissions{Admin: true},
		},
		BackendScopes: []users.BackendScope{{
			Path:        "/data",
			Permissions: users.SourceFilePermissions{Configured: true},
		}},
	}
	if SyncEnforcedSourcePermissionsOntoUser(u, defaults, enforced) {
		t.Fatal("expected enforced source sync to skip admin users")
	}
}

func TestValidateUserScopePermissionsAgainstEnforced_rejectsMismatch(t *testing.T) {
	defaults := users.SourceFilePermissions{Create: true, Configured: true}
	enforced := SourceFilePermissionsEnforcement{Create: true}
	u := &users.User{
		FrontendUser: users.FrontendUser{Username: "alice"},
		BackendScopes: []users.BackendScope{{
			Path:        "/data",
			Permissions: users.SourceFilePermissions{Create: false, Configured: true},
		}},
	}
	if err := ValidateUserScopePermissionsAgainstEnforced(u, defaults, enforced); err == nil {
		t.Fatal("expected validation error for mismatched create permission")
	}
}

func TestValidateSelfUserUpdateScopesNotEnforced_blocksScopesPatch(t *testing.T) {
	enforced := SourceFilePermissionsEnforcement{Create: true}
	if err := ValidateSelfUserUpdateScopesNotEnforced([]string{"scopes"}, enforced); err == nil {
		t.Fatal("expected self scope update to be blocked when create is enforced")
	}
}
