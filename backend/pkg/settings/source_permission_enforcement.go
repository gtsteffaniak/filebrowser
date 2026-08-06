package settings

import (
	"encoding/json"
	"fmt"
	"strings"

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

type sourcePermissionField struct {
	name     string
	field    func(*users.SourceFilePermissions) *bool
	enforced func(SourceFilePermissionsEnforcement) bool
}

var sourcePermissionFields = []sourcePermissionField{
	{"view", func(p *users.SourceFilePermissions) *bool { return &p.View }, func(e SourceFilePermissionsEnforcement) bool { return e.View }},
	{"download", func(p *users.SourceFilePermissions) *bool { return &p.Download }, func(e SourceFilePermissionsEnforcement) bool { return e.Download }},
	{"modify", func(p *users.SourceFilePermissions) *bool { return &p.Modify }, func(e SourceFilePermissionsEnforcement) bool { return e.Modify }},
	{"create", func(p *users.SourceFilePermissions) *bool { return &p.Create }, func(e SourceFilePermissionsEnforcement) bool { return e.Create }},
	{"delete", func(p *users.SourceFilePermissions) *bool { return &p.Delete }, func(e SourceFilePermissionsEnforcement) bool { return e.Delete }},
}

// EnforcedSourcePermissionFlags returns permission flag names with enforcement enabled.
func EnforcedSourcePermissionFlags(e SourceFilePermissionsEnforcement) map[string]struct{} {
	out := make(map[string]struct{})
	for _, f := range sourcePermissionFields {
		if f.enforced(e) {
			out[f.name] = struct{}{}
		}
	}
	return out
}

func sourcePermissionValue(perms users.SourceFilePermissions, flag string) bool {
	for _, f := range sourcePermissionFields {
		if f.name == flag {
			return *f.field(&perms)
		}
	}
	return false
}

func applyEnforcedSourcePermissionFlag(perms *users.SourceFilePermissions, flag string, def users.SourceFilePermissions) {
	for _, f := range sourcePermissionFields {
		if f.name == flag {
			*f.field(perms) = *f.field(&def)
			return
		}
	}
}

func setSourcePermissionFlag(perms *users.SourceFilePermissions, flag string, value bool) {
	for _, f := range sourcePermissionFields {
		if f.name == flag {
			*f.field(perms) = value
			return
		}
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
	if u == nil || u.Username == users.AnonymousUserName || !EnforcementAppliesToUser(u) {
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
	if u == nil || u.Username == users.AnonymousUserName || !EnforcementAppliesToUser(u) {
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
	if u == nil || u.Username == "" || u.Username == users.AnonymousUserName || !EnforcementAppliesToUser(u) {
		return nil
	}
	flags := EnforcedSourcePermissionFlags(enforced)
	if len(flags) == 0 {
		return nil
	}
	normalized := NormalizeSourceFilePermissions(defaults)
	for _, scope := range u.BackendScopes {
		for flag := range flags {
			expected := sourcePermissionValue(normalized, flag)
			actual := sourcePermissionValue(scope.Permissions, flag)
			if expected != actual {
				return ErrEnforcedUserValueMismatch{Path: fmt.Sprintf("scopes.%s.%s", scope.Path, flag)}
			}
		}
	}
	return nil
}

// ValidateSelfUserUpdateScopesNotEnforced rejects self-service scope updates when any permission is enforced.
func ValidateSelfUserUpdateScopesNotEnforced(which []string, enforced SourceFilePermissionsEnforcement, actor *users.User) error {
	if !EnforcementAppliesToUser(actor) {
		return nil
	}
	if len(EnforcedSourcePermissionFlags(enforced)) == 0 {
		return nil
	}
	for _, field := range which {
		f := strings.TrimSpace(field)
		if strings.EqualFold(f, "scopes") ||
			strings.EqualFold(f, "sourcePermissions") ||
			strings.EqualFold(f, "backendScopes") {
			return ErrEnforcedUserField{Path: "scopes"}
		}
	}
	return nil
}
