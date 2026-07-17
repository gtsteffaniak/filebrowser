package users

// EnsureSourcePermissionsForScopes repairs per-source permissions on each BackendScope.
// Re-keys legacy map/name entries and seeds missing scopes.
// Returns true if the user was modified.
func EnsureSourcePermissionsForScopes(user *User, defaults, adminDefaults SourceFilePermissions) bool {
	if user.Version < SourcePermissionsMigrationVersion {
		return false
	}
	changed := MergeLegacySourcePermissionsIntoScopes(user)
	seed := defaults
	if user.Permissions.Admin {
		seed = adminDefaults
	}
	if sourceConfig != nil {
		for key, perms := range user.BackendSourcePermissions {
			source, ok := sourceConfig.GetSourceByName(key)
			if !ok {
				continue
			}
			if _, atPath := user.BackendSourcePermissions[source.Path]; atPath && key == source.Path {
				continue
			}
			if user.BackendSourcePermissions == nil {
				user.BackendSourcePermissions = make(map[string]SourceFilePermissions)
			}
			user.BackendSourcePermissions[source.Path] = perms
			if key != source.Path {
				delete(user.BackendSourcePermissions, key)
			}
			changed = true
		}
	}
	for i, scope := range user.BackendScopes {
		if !scope.Permissions.IsUnset() {
			continue
		}
		if legacy, ok := user.BackendSourcePermissions[scope.Path]; ok {
			user.BackendScopes[i].Permissions = legacy
		} else {
			user.BackendScopes[i].Permissions = seed
		}
		changed = true
	}
	if changed {
		SyncBackendSourcePermissionsMap(user)
	}
	return changed
}
