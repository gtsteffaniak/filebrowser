package fileutils

import (
	"io/fs"
	"net/http"
)

var (
	assetFs fs.FS
)

// Custom dirFS to handle both embedded and non-embedded file systems
type dirFS struct {
	http.Dir
}

// Implement the Open method for dirFS, which wraps http.Dir
func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

// InitAssetFS initializes the asset filesystem for the application
// This should be called once during startup before http or preview services start
func InitAssetFS(embeddedAssets fs.FS, useEmbedded bool) {
	if useEmbedded {
		assetFs = embeddedAssets
	} else {
		// Dev mode: Serve files from http/dist directory
		assetFs = dirFS{Dir: http.Dir("http/dist")}
	}
}

// GetAssetFS returns the initialized asset filesystem
func GetAssetFS() fs.FS {
	return assetFs
}
