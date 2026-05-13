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

// APIScopesToBackend maps json "scopes" payloads (source display name or filesystem path + scope path)
func APIScopesToBackend(apiScopes []FrontendScope) ([]BackendScope, error) {
	if len(apiScopes) == 0 {
		return []BackendScope{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newScopes := []BackendScope{}
	for _, scope := range apiScopes {
		// Check if its the name of a source and convert it to a path
		source, ok := sourceConfig.GetSourceByName(scope.Name)
		if !ok {
			continue
		}
		if scope.Scope == "" {
			scope.Scope = source.DefaultUserScope
		}
		scope.Scope = normalizeScope(scope.Scope)
		newScopes = append(newScopes, BackendScope{
			Path:  source.Path,
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

// GetFrontendScopes returns API-style scopes from BackendScopes only (source display names).
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
		newScopes = append(newScopes, FrontendScope{
			Name:  source.Name,
			Scope: scope.Scope,
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
