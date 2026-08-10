//go:generate go run ./tools/yaml.go -input=pkg/settings/settings.go -output=config.generated.yaml
package settings

import (
	"crypto/rand"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

// boolPtr returns a pointer to a bool value
func boolPtr(b bool) *bool {
	return &b
}

// boolValueOrDefault returns the value of a bool pointer, or the default if nil
func boolValueOrDefault(ptr *bool, defaultValue bool) bool {
	if ptr != nil {
		return *ptr
	}
	return defaultValue
}

// ConvertPermissionsToUsers converts account permission defaults to users.Permissions (global only).
func ConvertPermissionsToUsers(p UserDefaultsAccountPermissions) users.Permissions {
	return users.Permissions{
		Api:      p.Api,
		Admin:    p.Admin,
		Share:    p.Share,
		Realtime: p.Realtime,
	}
}

const DefaultUsersHomeBasePath = "/users"

// AuthMethod describes an authentication method.
type AuthMethod string

// GenerateKey generates a key of 512 bits.
func GenerateKey() ([]byte, error) {
	b := make([]byte, 64)
	_, err := rand.Read(b)
	// Note that err == nil only if we read len(b) bytes.
	if err != nil {
		return nil, err
	}

	return b, nil
}

func GetSettingsConfig(nameType string, Value string) string {
	return nameType + Value
}

func AdminPerms() users.Permissions {
	return users.Permissions{
		Share:    true,
		Admin:    true,
		Api:      true,
		Realtime: false,
	}
}

// AdminSourceFilePermissions returns full per-source file permissions for admin users.
func AdminSourceFilePermissions() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View:     true,
		Download: true,
		Modify:   true,
		Delete:   true,
		Create:   true,
	}
}

// ApplyUserDefaults applies Config.UserDefaults to a user (tests and legacy callers).
func ApplyUserDefaults(u *users.User) {
	ApplyUserDefaultsFrom(u, Config.UserDefaults)
}

// ApplyUserDefaultsFrom applies the given defaults template to a user.
// Keep this in sync with [UserDefaults]: every user-facing field there should be copied
// here (except DefaultScopes, which is wired through source config rather than this helper).
func ApplyUserDefaultsFrom(u *users.User, d UserDefaults) {
	ApplyFullProfileFromDefaults(u, d)

	if u.Username == "anonymous" {
		return
	}

	// Global permissions (admin, api, share, realtime)
	u.Permissions.Api = d.Account.Permissions.Api
	u.Permissions.Admin = d.Account.Permissions.Admin
	u.Permissions.Share = d.Account.Permissions.Share
	u.Permissions.Realtime = d.Account.Permissions.Realtime

	sourceDefaults := DefaultSourceFilePermissions()

	if len(u.BackendScopes) == 0 {
		for _, source := range Config.Server.Sources {
			if source.Config.DefaultEnabled {
				u.BackendScopes = append(u.BackendScopes, users.BackendScope{
					Path:  source.Path,
					Scope: source.Config.DefaultUserScope,
				})
				if len(u.SidebarLinks) == 0 {
					u.SidebarLinks = append(u.SidebarLinks, users.SidebarLink{
						Name:       source.Name,
						Category:   "source",
						Target:     "/",
						Icon:       "",
						SourceName: source.Path,
					})
				}
			}
		}
	}

	if u.BackendSourcePermissions == nil {
		u.BackendSourcePermissions = make(map[string]users.SourceFilePermissions)
	}
	for i, scope := range u.BackendScopes {
		if _, ok := u.BackendSourcePermissions[scope.Path]; !ok {
			perms := sourceDefaults
			if u.Permissions.Admin {
				perms = AdminSourceFilePermissions()
			}
			u.BackendScopes[i].Permissions = perms
			u.BackendSourcePermissions[scope.Path] = perms
		} else if u.BackendScopes[i].Permissions.IsUnset() {
			u.BackendScopes[i].Permissions = u.BackendSourcePermissions[scope.Path]
		}
	}

	ExpandBackendScopesForCreateUserDir(u)

	if u.LoginMethod == "" && d.Account.LoginMethod != "" {
		u.LoginMethod = users.LoginMethod(d.Account.LoginMethod)
	}

	u.Version = users.CurrentUserMigrationVersion
}
