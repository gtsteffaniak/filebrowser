package settings

import (
	"fmt"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/go-logger/logger"
)

func ConvertToBackendScopes(scopes []users.SourceScope) ([]users.SourceScope, error) {
	if len(scopes) == 0 {
		return Config.UserDefaults.DefaultScopes, nil
	}
	newScopes := []users.SourceScope{}
	for _, scope := range scopes {

		// first check if its already a path name and keep it
		source, ok := Config.Server.SourceMap[scope.Name]
		if ok {
			if scope.Scope == "" {
				scope.Scope = source.Config.DefaultUserScope
			}
			if !strings.HasPrefix(scope.Scope, "/") {
				scope.Scope = "/" + scope.Scope
			}
			if scope.Scope != "/" && strings.HasSuffix(scope.Scope, "/") {
				scope.Scope = strings.TrimSuffix(scope.Scope, "/")
			}
			newScopes = append(newScopes, users.SourceScope{
				Name:  source.Path, // backend name is path
				Scope: scope.Scope,
			})
			continue
		}

		// check if its the name of a source and convert it to a path
		source, ok = Config.Server.NameToSource[scope.Name]
		if !ok {
			// source might no longer be configured
			continue
		}
		if scope.Scope == "" {
			scope.Scope = source.Config.DefaultUserScope
		}
		if !strings.HasPrefix(scope.Scope, "/") {
			scope.Scope = "/" + scope.Scope
		}
		if scope.Scope != "/" && strings.HasSuffix(scope.Scope, "/") {
			scope.Scope = strings.TrimSuffix(scope.Scope, "/")
		}
		newScopes = append(newScopes, users.SourceScope{
			Name:  source.Path, // backend name is path
			Scope: scope.Scope,
		})
	}
	return newScopes, nil
}

func ConvertToFrontendScopes(scopes []users.SourceScope) []users.SourceScope {
	newScopes := []users.SourceScope{}
	for _, scope := range scopes {
		if source, ok := Config.Server.SourceMap[scope.Name]; ok {
			// Replace scope.Name with source.Path while keeping the same Scope value
			newScopes = append(newScopes, users.SourceScope{
				Name:  source.Name,
				Scope: scope.Scope,
			})
		}
	}
	return newScopes
}

func ConvertToFrontendSidebarLinks(links []users.SidebarLink) []users.SidebarLink {
	newLinks := []users.SidebarLink{}
	for _, link := range links {
		// For source links, validate that the source still exists
		if link.Category == "source" {
			if link.SourceName == "" {
				logger.Warningf("source link missing sourceName: %v", link.Name)
				continue
			}
			// Check if source exists
			sourceInfo, ok := Config.Server.NameToSource[link.SourceName]
			if !ok {
				logger.Warningf("source not found: %v (link name: %v)", link.SourceName, link.Name)
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

func ConvertToBackendSidebarLinks(links []users.SidebarLink) ([]users.SidebarLink, error) {
	newLinks := []users.SidebarLink{}
	for _, link := range links {
		// For source links, validate that the source exists using SourceName
		if link.Category == "source" {
			if link.SourceName == "" {
				return nil, fmt.Errorf("source link missing sourceName (link name: %v)", link.Name)
			}
			// Validate source exists
			sourceInfo, ok := Config.Server.NameToSource[link.SourceName]
			if !ok {
				return nil, fmt.Errorf("source not found: %v (link name: %v)", link.SourceName, link.Name)
			}
			link.SourceName = sourceInfo.Path // if name chages keep link alive
		}
		// Store the link as-is with all fields preserved
		newLinks = append(newLinks, link)
	}
	return newLinks, nil
}

func HasSourceByPath(scopes []users.SourceScope, sourcePath string) bool {
	for _, scope := range scopes {
		if scope.Name == sourcePath {
			return true
		}
	}
	return false
}

func GetScopeFromSourceName(scopes []users.SourceScope, sourceName string) (string, error) {
	source, ok := Config.Server.NameToSource[sourceName]
	if !ok {
		logger.Debug("Could not get scope from source name: ", sourceName)
		return "", fmt.Errorf("source with name not found %v", sourceName)
	}
	for _, scope := range scopes {
		if scope.Name == source.Path {
			return scope.Scope, nil
		}
	}
	logger.Debugf("scope not found for source %v", sourceName)
	return "", fmt.Errorf("scope not found for source %v", sourceName)
}

func GetScopeFromSourcePath(scopes []users.SourceScope, sourcePath string) (string, error) {
	for _, scope := range scopes {
		if scope.Name == sourcePath {
			return scope.Scope, nil
		}
	}
	return "", fmt.Errorf("scope not found for source %v", sourcePath)
}

// assumes backend style scopes
func GetSources(u *users.User) []string {
	sources := []string{}

	// preserves order of sources
	for _, source := range Config.Server.Sources {
		_, err := GetScopeFromSourcePath(u.Scopes, source.Path)
		if err != nil {
			logger.Warningf("could not get scope for source %v: %v", source.Path, err)
			continue
		}
		sources = append(sources, source.Name)
	}
	return sources
}
