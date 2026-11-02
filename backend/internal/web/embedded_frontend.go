package web

import (
	"io/fs"
)

// embeddedFrontendInDist reports whether dist contains a built frontend bundle.
func embeddedFrontendInDist(dist fs.FS) bool {
	_, err := fs.Stat(dist, "public/index.html")
	return err == nil
}

// EmbeddedFrontendAvailable reports whether the binary was built with a full frontend
// bundle in dist (as opposed to the placeholder .gitkeep used for bare go build/test).
func EmbeddedFrontendAvailable() bool {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return false
	}
	return embeddedFrontendInDist(sub)
}
