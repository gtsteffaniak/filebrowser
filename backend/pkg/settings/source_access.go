package settings

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// BuiltinDefaultSourceFilePermissions is used when no source access defaults are configured.
func BuiltinDefaultSourceFilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   false,
		Create:   false,
		Delete:   false,
	}
}

// NormalizeSourceFilePermissions returns built-in defaults when all flags are unset.
func NormalizeSourceFilePermissions(p users.SourceFilePermissions) users.SourceFilePermissions {
	if p.IsUnset() {
		return BuiltinDefaultSourceFilePermissions()
	}
	return p
}

// ApplySourceAccessDefaultsToAllSources copies the same template onto every configured source.
func ApplySourceAccessDefaultsToAllSources(perms users.SourceFilePermissions) {
	p := NormalizeSourceFilePermissions(perms)
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		src.Config.DefaultPermissions = p
	}
}

// DefaultSourceFilePermissions returns the effective global source access defaults.
func DefaultSourceFilePermissions() users.SourceFilePermissions {
	for _, src := range Config.Server.Sources {
		if src == nil {
			continue
		}
		if !src.Config.DefaultPermissions.IsUnset() {
			return NormalizeSourceFilePermissions(src.Config.DefaultPermissions)
		}
	}
	return BuiltinDefaultSourceFilePermissions()
}

// SourceFilePermissionsFromLegacyUserDefaults maps deprecated user-defaults permission fields.
func SourceFilePermissionsFromLegacyUserDefaults(d UserDefaults) users.SourceFilePermissions {
	p := d.Permissions
	download := true
	if p.Download != nil {
		download = *p.Download
	}
	return users.SourceFilePermissions{
		View:     true,
		Download: download,
		Modify:   p.Modify,
		Create:   p.Create,
		Delete:   p.Delete,
	}
}
