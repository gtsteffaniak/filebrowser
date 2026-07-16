package users

import (
	"fmt"
	"strings"
)

// ResolveSourceKey maps a string that is either:
//   - the configured source filesystem path (canonical Bolt / in-memory form for
//     SourceScope.Name and source-type SidebarLink.SourceName), or
//   - the source display name (JSON form from clients).
//
// Path is checked first so absolute paths are unambiguous. Used on write paths
// (normalize to Path) and read paths (normalize API to display Name).
func ResolveSourceKey(key string) (SourceInfo, bool) {
	if sourceConfig == nil || key == "" {
		return SourceInfo{}, false
	}
	if info, ok := sourceConfig.GetSourceByPath(key); ok {
		return info, true
	}
	return sourceConfig.GetSourceByName(key)
}

// GetSourceNames returns all source names the user has access to (assumes backend-style scopes)
func (u *User) GetSourceNames() []string {
	if sourceConfig == nil {
		return []string{}
	}

	allSources := sourceConfig.GetAllSources()
	sources := []string{}
	// Preserves order of sources
	for _, source := range allSources {
		_, err := u.GetScopeForSourcePath(source.Path)
		if err == nil {
			sources = append(sources, source.Name)
		}
	}
	return sources
}

// APIScopesToBackend maps json "scopes" payloads (source display name or filesystem path + scope path + permissions).
// Omitted scope permissions become deny-all; callers must pre-fill defaults before conversion when needed.
func APIScopesToBackend(apiScopes []FrontendScope) ([]BackendScope, error) {
	if len(apiScopes) == 0 {
		return []BackendScope{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newScopes := []BackendScope{}
	for _, scope := range apiScopes {
		source, ok := sourceConfig.GetSourceByName(scope.Name)
		if !ok {
			continue
		}
		if scope.Scope == "" {
			scope.Scope = source.DefaultUserScope
		}
		scope.Scope = normalizeScope(scope.Scope)
		newScopes = append(newScopes, BackendScope{
			Path:        source.Path,
			Scope:       scope.Scope,
			Permissions: frontendScopePermissions(scope),
		})
	}
	return newScopes, nil
}

// GetFrontendScopes returns API-style scopes from BackendScopes (source display names + permissions).
func (u *User) GetFrontendScopes() []FrontendScope {
	if sourceConfig == nil {
		return []FrontendScope{}
	}

	newScopes := []FrontendScope{}
	for _, scope := range u.BackendScopes {
		source, ok := sourceConfig.GetSourceByPath(scope.Path)
		if !ok {
			continue
		}
		perms := scope.Permissions
		newScopes = append(newScopes, FrontendScope{
			Name:        source.Name,
			Scope:       scope.Scope,
			Permissions: &perms,
		})
	}
	return newScopes
}

// GetBackendSidebarLinks normalizes source links for Bolt: SourceName is the filesystem path.
func (u *User) GetBackendSidebarLinks() ([]SidebarLink, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newLinks := []SidebarLink{}
	for _, link := range u.SidebarLinks {
		if strings.HasPrefix(link.Category, "source") {
			if link.SourceName == "" {
				return nil, fmt.Errorf("source link missing sourceName (link name: %v)", link.Name)
			}
			// Validate source exists
			sourceInfo, ok := sourceConfig.GetSourceByName(link.SourceName)
			sourceInfo2, ok2 := sourceConfig.GetSourceByPath(link.SourceName)
			if !ok && !ok2 {
				return nil, fmt.Errorf("source not found: %v (link name: %v)", link.SourceName, link.Name)
			}
			if ok {
				link.SourceName = sourceInfo.Path
			} else {
				link.SourceName = sourceInfo2.Path
			}
		}
		newLinks = append(newLinks, link)
	}
	return newLinks, nil
}

// normalizeScope ensures scope starts with / and doesn't end with / (except for root)
func normalizeScope(scope string) string {
	if !strings.HasPrefix(scope, "/") {
		scope = "/" + scope
	}
	if scope != "/" && strings.HasSuffix(scope, "/") {
		scope = strings.TrimSuffix(scope, "/")
	}
	return scope
}

// APISourcePermsToBackend maps API sourcePermissions (display name keys) to backend path keys.
func APISourcePermsToBackend(apiPerms map[string]SourceFilePermissions) (map[string]SourceFilePermissions, error) {
	if len(apiPerms) == 0 {
		return map[string]SourceFilePermissions{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}
	out := make(map[string]SourceFilePermissions, len(apiPerms))
	for key, perms := range apiPerms {
		source, ok := ResolveSourceKey(key)
		if !ok {
			continue
		}
		out[source.Path] = perms
	}
	return out, nil
}

// GetFrontendSourcePermissions returns API-style sourcePermissions from BackendSourcePermissions.
func (u *User) GetFrontendSourcePermissions() map[string]SourceFilePermissions {
	if sourceConfig == nil || len(u.BackendSourcePermissions) == 0 {
		return map[string]SourceFilePermissions{}
	}
	out := make(map[string]SourceFilePermissions, len(u.BackendSourcePermissions))
	for path, perms := range u.BackendSourcePermissions {
		source, ok := sourceConfig.GetSourceByPath(path)
		if !ok {
			continue
		}
		out[source.Name] = perms
	}
	return out
}

// FilePermsForSourcePath returns per-source file permissions for a backend source path.
func (u *User) FilePermsForSourcePath(sourcePath string) (SourceFilePermissions, bool) {
	for _, scope := range u.BackendScopes {
		if scope.Path == sourcePath {
			perms := scope.Permissions
			if perms.IsUnset() {
				if u.BackendSourcePermissions != nil {
					if legacy, ok := u.BackendSourcePermissions[sourcePath]; ok {
						return legacy, true
					}
				}
			}
			return perms, true
		}
	}
	if u.BackendSourcePermissions != nil {
		perms, ok := u.BackendSourcePermissions[sourcePath]
		return perms, ok
	}
	return DenyAllSourceFilePermissions(), false
}

// FilePermsForSourceName resolves a source display name and returns per-source file permissions.
func (u *User) FilePermsForSourceName(sourceName string) (SourceFilePermissions, error) {
	if sourceNameResolver == nil {
		return DenyAllSourceFilePermissions(), fmt.Errorf("source name resolver not initialized")
	}
	sourcePath, err := sourceNameResolver(sourceName)
	if err != nil {
		return DenyAllSourceFilePermissions(), err
	}
	perms, ok := u.FilePermsForSourcePath(sourcePath)
	if !ok {
		return DenyAllSourceFilePermissions(), nil
	}
	return perms, nil
}

// IntersectSourceFilePermissions returns the intersection of two SourceFilePermissions (token caps).
func IntersectSourceFilePermissions(a, b SourceFilePermissions) SourceFilePermissions {
	return SourceFilePermissions{
		View:     a.View && b.View,
		Download: a.Download && b.Download,
		Modify:   a.Modify && b.Modify,
		Delete:   a.Delete && b.Delete,
		Create:   a.Create && b.Create,
	}
}
