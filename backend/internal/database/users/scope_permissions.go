package users

// GlobalPermissionsOnly returns API-facing global permissions (admin, api, share, realtime).
func GlobalPermissionsOnly(p Permissions) Permissions {
	return Permissions{
		Admin:    p.Admin,
		Api:      p.Api,
		Share:    p.Share,
		Realtime: p.Realtime,
	}
}

func frontendScopePermissions(fs FrontendScope) SourceFilePermissions {
	if fs.Permissions == nil {
		return DenyAllSourceFilePermissions()
	}
	return *fs.Permissions
}

// MergeLegacySourcePermissionsIntoScopes copies deprecated sourcePermissions / backendSourcePermissions
// into BackendScopes[].Permissions when scopes lack file permissions.
func MergeLegacySourcePermissionsIntoScopes(user *User) bool {
	if user == nil {
		return false
	}
	changed := false
	legacyByPath := user.BackendSourcePermissions
	if legacyByPath == nil {
		legacyByPath = make(map[string]SourceFilePermissions)
	}
	if sourceConfig != nil && len(user.SourcePermissions) > 0 {
		for key, perms := range user.SourcePermissions {
			source, ok := ResolveSourceKey(key)
			if !ok {
				continue
			}
			if _, exists := legacyByPath[source.Path]; !exists {
				legacyByPath[source.Path] = perms
				changed = true
			}
		}
	}
	for i, scope := range user.BackendScopes {
		if !scope.Permissions.IsUnset() {
			continue
		}
		if legacy, ok := legacyByPath[scope.Path]; ok {
			user.BackendScopes[i].Permissions = legacy
			changed = true
		}
	}
	if changed {
		SyncBackendSourcePermissionsMap(user)
	}
	return changed
}

// SyncBackendSourcePermissionsMap rebuilds the legacy path-keyed map from scope permissions.
func SyncBackendSourcePermissionsMap(user *User) {
	if user == nil {
		return
	}
	m := make(map[string]SourceFilePermissions, len(user.BackendScopes))
	for _, scope := range user.BackendScopes {
		m[scope.Path] = scope.Permissions
	}
	user.BackendSourcePermissions = m
}
