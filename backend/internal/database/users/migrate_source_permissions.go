package users

// SourcePermissionsMigrationVersion is the user version after per-source file permissions migration.
const SourcePermissionsMigrationVersion = 4

// MigrateToSourcePermissions copies global file-operation permissions to each BackendScope source.
// Returns true if the user was modified. Idempotent when Version >= SourcePermissionsMigrationVersion.
func MigrateToSourcePermissions(user *User) bool {
	if user.Version >= SourcePermissionsMigrationVersion {
		return false
	}
	legacy := SourceFilePermissions{
		View:     true,
		Download: user.Permissions.Download,
		Modify:   user.Permissions.Modify,
		Delete:   user.Permissions.Delete,
		Create:   user.Permissions.Create,
	}
	for i, scope := range user.BackendScopes {
		user.BackendScopes[i].Permissions = legacy
		_ = scope
	}
	SyncBackendSourcePermissionsMap(user)
	user.Permissions = GlobalPermissionsOnly(user.Permissions)
	user.SourcePermissions = nil
	user.Version = SourcePermissionsMigrationVersion
	return true
}

// SeedSourcePermissionsForPath adds default per-source permissions for a newly assigned source scope.
func SeedSourcePermissionsForPath(user *User, sourcePath string, defaults SourceFilePermissions) bool {
	if user.Version < SourcePermissionsMigrationVersion {
		return false
	}
	for i, scope := range user.BackendScopes {
		if scope.Path != sourcePath {
			continue
		}
		if !scope.Permissions.IsUnset() {
			return false
		}
		user.BackendScopes[i].Permissions = defaults
		SyncBackendSourcePermissionsMap(user)
		return true
	}
	return false
}
