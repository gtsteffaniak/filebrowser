package settings

import (
	"fmt"

	"github.com/gtsteffaniak/filebrowser/backend/database/users"
)

// InitializeUserResolvers sets up the global resolvers and config providers in the users package
// This should be called once during application initialization
func InitializeUserResolvers() {
	// Set up the source name resolver
	users.SetSourceNameResolver(func(sourceName string) (string, error) {
		source, ok := Config.Server.NameToSource[sourceName]
		if !ok {
			return "", fmt.Errorf("source name not found: %s", sourceName)
		}
		return source.Path, nil
	})

	// Set up the source config provider
	users.SetSourceConfig(&users.SourceConfigProvider{
		GetSourceByPath: func(path string) (users.SourceInfo, bool) {
			source, ok := Config.Server.SourceMap[path]
			if !ok {
				return users.SourceInfo{}, false
			}
			return users.SourceInfo{
				Path:             source.Path,
				Name:             source.Name,
				DefaultUserScope: source.Config.DefaultUserScope,
			}, true
		},
		GetSourceByName: func(name string) (users.SourceInfo, bool) {
			source, ok := Config.Server.NameToSource[name]
			if !ok {
				return users.SourceInfo{}, false
			}
			return users.SourceInfo{
				Path:             source.Path,
				Name:             source.Name,
				DefaultUserScope: source.Config.DefaultUserScope,
			}, true
		},
		GetAllSources: func() []users.SourceInfo {
			sources := make([]users.SourceInfo, 0, len(Config.Server.Sources))
			for _, source := range Config.Server.Sources {
				sources = append(sources, users.SourceInfo{
					Path:             source.Path,
					Name:             source.Name,
					DefaultUserScope: source.Config.DefaultUserScope,
				})
			}
			return sources
		},
		GetDefaultScopes: func() []users.SourceScope {
			return Config.UserDefaults.DefaultScopes
		},
	})
}
