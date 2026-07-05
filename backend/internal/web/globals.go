package web

import (
	"io/fs"
)

var assetFs fs.FS

// InitGlobals sets package-level dependencies used by handlers and middleware.
func InitGlobals(assets fs.FS) {
	assetFs = assets
}
