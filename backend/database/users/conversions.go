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

	sources := []string{}
	allSources := sourceConfig.GetAllSources()

	// Preserves order of sources
	for _, source := range allSources {
		_, err := u.GetScopeForSourcePath(source.Path)
		if err == nil {
			sources = append(sources, source.Name)
		}
	}
	return sources
}

// GetBackendScopes normalizes scopes for Bolt: SourceScope.Name is always the source filesystem path.
// Incoming rows (API) may use display name or path; legacy rows may use either.
func (u *User) GetBackendScopes() ([]SourceScope, error) {
	if len(u.Scopes) == 0 {
		return []SourceScope{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newScopes := []SourceScope{}
	for _, scope := range u.Scopes {
		source, ok := ResolveSourceKey(scope.Name)
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

// GetFrontendScopes converts scopes for JSON clients: SourceScope.Name is always the source display name.
// Assumes Bolt stores paths (GetBackendScopes); unknown keys are omitted.
func (u *User) GetFrontendScopes() []SourceScope {
	if sourceConfig == nil {
		return []SourceScope{}
	}

	newScopes := []SourceScope{}
	for _, scope := range u.Scopes {
		source, ok := ResolveSourceKey(scope.Name)
		if !ok {
			continue
		}
		newScopes = append(newScopes, SourceScope{
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
