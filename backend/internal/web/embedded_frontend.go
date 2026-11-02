package web

import (
	"io/fs"
)

// EmbeddedFrontendAvailable reports whether the binary was built with a full frontend
// bundle in dist (as opposed to the placeholder .gitkeep used for bare go build/test).
func EmbeddedFrontendAvailable() bool {
	sub, err := fs.Sub(assets, "dist")
	if err != nil {
		return false
	}
	_, err = fs.Stat(sub, "public/index.html")
	return err == nil
}
