package fileutils

import (
	"io/fs"
	"net/http"

	"github.com/gtsteffaniak/go-logger/logger"
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
func InitAssetFS(embeddedAssets fs.FS, useEmbedded bool, devMode bool) {
	if useEmbedded {
		assetFs = embeddedAssets
	} else if devMode {
		// Dev mode: serve frontend source assets (templates, fonts, icons)
		assetFs = dirFS{Dir: http.Dir("../frontend/public")}
	} else {
		// Non-dev disk build (e.g. Docker): dist is shipped beside the binary. Preload it
		// into memory when possible; otherwise serve from disk.
		diskAssets := dirFS{Dir: http.Dir("internal/web/dist")}
		cached, err := loadMemFS(diskAssets)
		if err != nil {
			logger.Warningf("could not preload frontend assets into memory, serving from disk: %v", err)
			assetFs = diskAssets
		} else {
			logger.Debugf("preloaded %d frontend assets (%d bytes) into memory", cached.Len(), cached.Size())
			assetFs = cached
		}
	}
}

// GetAssetFS returns the initialized asset filesystem
func GetAssetFS() fs.FS {
	return assetFs
}
