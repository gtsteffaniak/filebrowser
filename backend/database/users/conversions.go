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
func APIScopesToBackend(apiScopes []SourceScope) ([]SourceScope, error) {
	if len(apiScopes) == 0 {
		return []SourceScope{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newScopes := []SourceScope{}
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
		newScopes = append(newScopes, SourceScope{
			Name:  source.Path,
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

// GetFrontendScopes returns API-style scopes from BackendScopes only (source display names).
func (u *User) GetFrontendScopes() []SourceScope {
	if sourceConfig == nil {
		return []SourceScope{}
	}

	newScopes := []SourceScope{}
	for _, scope := range u.BackendScopes {
		if source, ok := sourceConfig.GetSourceByPath(scope.Name); ok {
			// Replace scope.Name with source.Name while keeping the same Scope value
			newScopes = append(newScopes, SourceScope{
				Name:  source.Name,
				Scope: scope.Scope,
			})
		}
		newScopes = append(newScopes, SourceScope{
			Name:  scope.Name,
			Scope: scope.Scope,
		})
	}
	return newScopes
}

// GetFrontendSidebarLinks converts the user's sidebar links from backend-style to frontend-style
// Converts source paths to source names for the frontend
func (u *User) GetFrontendSidebarLinks() []SidebarLink {
	if sourceConfig == nil {
		return []SidebarLink{}
	}

	newLinks := []SidebarLink{}
	for _, link := range u.SidebarLinks {
		// For source links, validate that the source still exists
		if strings.HasPrefix(link.Category, "source") {
			if link.SourceName == "" {
				continue
			}
			// Check if source exists
			sourceInfo, ok := sourceConfig.GetSourceByPath(link.SourceName)
			if !ok {
				continue
			}
			link.SourceName = sourceInfo.Name
		}
		// For share links, just pass through (shares are validated separately)
		// For all other links, pass through as-is
		newLinks = append(newLinks, link)
	}
	return newLinks
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
