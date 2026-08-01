package settings

import (
	"encoding/json"
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// SourceFilePermissionsEnforcement marks which source file permission defaults are enforced for all users.
type SourceFilePermissionsEnforcement struct {
	View     bool `json:"view,omitempty"`
	Download bool `json:"download,omitempty"`
	Modify   bool `json:"modify,omitempty"`
	Create   bool `json:"create,omitempty"`
	Delete   bool `json:"delete,omitempty"`
}

// EnforcedSourcePermissionFlags returns permission flag names with enforcement enabled.
func EnforcedSourcePermissionFlags(e SourceFilePermissionsEnforcement) map[string]struct{} {
	out := make(map[string]struct{})
	if e.View {
		out["view"] = struct{}{}
	}
	if e.Download {
		out["download"] = struct{}{}
	}
	if e.Modify {
		out["modify"] = struct{}{}
	}
	if e.Create {
		out["create"] = struct{}{}
	}
	if e.Delete {
		out["delete"] = struct{}{}
	}
	return out
}

func sourcePermissionDefaultValue(def users.SourceFilePermissions, flag string) bool {
	switch flag {
	case "view":
		return def.View
	case "download":
		return def.Download
	case "modify":
		return def.Modify
	case "create":
		return def.Create
	case "delete":
		return def.Delete
	default:
		return false
	}
}

func sourcePermissionActualValue(perms users.SourceFilePermissions, flag string) bool {
	switch flag {
	case "view":
		return perms.View
	case "download":
		return perms.Download
	case "modify":
		return perms.Modify
	case "create":
		return perms.Create
	case "delete":
		return perms.Delete
	default:
		return false
	}
}

func applyEnforcedSourcePermissionFlag(perms *users.SourceFilePermissions, flag string, def users.SourceFilePermissions) {
	switch flag {
	case "view":
		perms.View = def.View
	case "download":
		perms.Download = def.Download
	case "modify":
		perms.Modify = def.Modify
	case "create":
		perms.Create = def.Create
	case "delete":
		perms.Delete = def.Delete
	}
}

// MergeSourceEnforcedPatchJSON merges a partial JSON patch into base source permission enforcement.
func MergeSourceEnforcedPatchJSON(base SourceFilePermissionsEnforcement, patchJSON []byte) (SourceFilePermissionsEnforcement, error) {
	baseBytes, err := json.Marshal(base)
	if err != nil {
		return SourceFilePermissionsEnforcement{}, err
	}
	mergedBytes, err := mergeUserDefaultsJSON(baseBytes, patchJSON)
	if err != nil {
		return SourceFilePermissionsEnforcement{}, err
	}
	var merged SourceFilePermissionsEnforcement
	if err := json.Unmarshal(mergedBytes, &merged); err != nil {
		return SourceFilePermissionsEnforcement{}, err
	}
	return merged, nil
}

// ApplyEnforcedSourcePermissionsFrom sets enforced permission bits on every scope from defaults.
func ApplyEnforcedSourcePermissionsFrom(u *users.User, defaults users.SourceFilePermissions, enforced SourceFilePermissionsEnforcement) {
	if !EnforcementAppliesToUser(u) || u == nil || u.Username == users.AnonymousUserName {
		return
	}
	flags := EnforcedSourcePermissionFlags(enforced)
	if len(flags) == 0 {
		return
	}
	normalized := NormalizeSourceFilePermissions(defaults)
	for i := range u.BackendScopes {
		for flag := range flags {
			applyEnforcedSourcePermissionFlag(&u.BackendScopes[i].Permissions, flag, normalized)
		}
		u.BackendScopes[i].Permissions = users.MarkSourceFilePermissionsConfigured(u.BackendScopes[i].Permissions)
	}
	users.SyncBackendSourcePermissionsMap(u)
}

// SyncEnforcedSourcePermissionsOntoUser applies enforced defaults onto scope permissions. Returns true if changed.
func SyncEnforcedSourcePermissionsOntoUser(u *users.User, defaults users.SourceFilePermissions, enforced SourceFilePermissionsEnforcement) bool {
	if !EnforcementAppliesToUser(u) || u == nil || u.Username == users.AnonymousUserName {
		return false
	}
	flags := EnforcedSourcePermissionFlags(enforced)
	if len(flags) == 0 {
		return false
	}
	normalized := NormalizeSourceFilePermissions(defaults)
	changed := false
	for i := range u.BackendScopes {
		before := u.BackendScopes[i].Permissions
		for flag := range flags {
			applyEnforcedSourcePermissionFlag(&u.BackendScopes[i].Permissions, flag, normalized)
		}
		u.BackendScopes[i].Permissions = users.MarkSourceFilePermissionsConfigured(u.BackendScopes[i].Permissions)
		if u.BackendScopes[i].Permissions != before {
			changed = true
		}
	}
	if changed {
		users.SyncBackendSourcePermissionsMap(u)
	}
	return changed
}

// ValidateUserScopePermissionsAgainstEnforced rejects users whose enforced scope permissions differ from defaults.
func ValidateUserScopePermissionsAgainstEnforced(u *users.User, defaults users.SourceFilePermissions, enforced SourceFilePermissionsEnforcement) error {
	if !EnforcementAppliesToUser(u) {
		return nil
	}
	flags := EnforcedSourcePermissionFlags(enforced)
	if len(flags) == 0 || u == nil || u.Username == "" || u.Username == users.AnonymousUserName {
		return nil
	}
	normalized := NormalizeSourceFilePermissions(defaults)
	for _, scope := range u.BackendScopes {
		for flag := range flags {
			expected := sourcePermissionDefaultValue(normalized, flag)
			actual := sourcePermissionActualValue(scope.Permissions, flag)
			if expected != actual {
				return ErrEnforcedUserValueMismatch{Path: fmt.Sprintf("scopes.%s.%s", scope.Path, flag)}
			}
		}
	}
	return nil
}

// ValidateSelfUserUpdateScopesNotEnforced rejects self-service scope updates when any permission is enforced.
func ValidateSelfUserUpdateScopesNotEnforced(which []string, enforced SourceFilePermissionsEnforcement) error {
	if len(EnforcedSourcePermissionFlags(enforced)) == 0 {
		return nil
	}
	for _, field := range which {
		if field == "scopes" || field == "sourcePermissions" || field == "backendScopes" {
			return ErrEnforcedUserField{Path: "scopes"}
		}
	}
	return nil
}
