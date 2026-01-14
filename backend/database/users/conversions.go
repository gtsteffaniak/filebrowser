package users

import (
	"fmt"
	"strings"
)

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

// GetBackendScopes converts the user's scopes from frontend-style to backend-style
func (u *User) GetBackendScopes() ([]SourceScope, error) {
	// Only convert scopes if they are not empty
	// Empty scopes during update should remain empty (not filled with defaults)
	if len(u.Scopes) == 0 {
		return []SourceScope{}, nil
	}
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newScopes := []SourceScope{}
	for _, scope := range u.Scopes {
		// First check if its already a path name and keep it
		source, ok := sourceConfig.GetSourceByPath(scope.Name)
		if ok {
			if scope.Scope == "" {
				scope.Scope = source.DefaultUserScope
			}
			scope.Scope = normalizeScope(scope.Scope)
			newScopes = append(newScopes, SourceScope{
				Name:  source.Path, // backend name is path
				Scope: scope.Scope,
			})
			continue
		}

		// Check if its the name of a source and convert it to a path
		source, ok = sourceConfig.GetSourceByName(scope.Name)
		if !ok {
			// source might no longer be configured
			continue
		}
		if scope.Scope == "" {
			scope.Scope = source.DefaultUserScope
		}
		scope.Scope = normalizeScope(scope.Scope)
		newScopes = append(newScopes, SourceScope{
			Name:  source.Path, // backend name is path
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

// GetFrontendScopes converts the user's scopes from backend-style to frontend-style
// Backend scopes use source paths, frontend scopes use source names
func (u *User) GetFrontendScopes() []SourceScope {
	if sourceConfig == nil {
		return []SourceScope{}
	}

	newScopes := []SourceScope{}
	for _, scope := range u.Scopes {
		if source, ok := sourceConfig.GetSourceByPath(scope.Name); ok {
			// Replace scope.Name with source.Name while keeping the same Scope value
			newScopes = append(newScopes, SourceScope{
				Name:  source.Name,
				Scope: scope.Scope,
			})
		}
	}
	return newScopes
}

// GetBackendSidebarLinks converts the user's sidebar links from frontend-style to backend-style
// Validates that sources exist and converts source names to paths
func (u *User) GetBackendSidebarLinks() ([]SidebarLink, error) {
	if sourceConfig == nil {
		return nil, fmt.Errorf("source config not initialized")
	}

	newLinks := []SidebarLink{}
	for _, link := range u.SidebarLinks {
		// For source links, validate that the source exists using SourceName
		if link.Category == "source" {
			if link.SourceName == "" {
				return nil, fmt.Errorf("source link missing sourceName (link name: %v)", link.Name)
			}
			// Validate source exists
			sourceInfo, ok := sourceConfig.GetSourceByName(link.SourceName)
			if !ok {
				return nil, fmt.Errorf("source not found: %v (link name: %v)", link.SourceName, link.Name)
			}
			link.SourceName = sourceInfo.Path // if name changes keep link alive
		}
		// Store the link as-is with all fields preserved
		newLinks = append(newLinks, link)
	}
	return newLinks, nil
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
		if link.Category == "source" {
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
