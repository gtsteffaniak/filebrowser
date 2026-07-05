package web

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
)

// Deps holds injected dependencies for HTTP handlers and middleware.
type Deps struct {
	Store *state.Store
	Files *files.Service
	Auth  *auth.Service
}

var runtimeDeps Deps

// SetDeps registers runtime dependencies for handlers (called once from cmd startup).
func SetDeps(d Deps) {
	runtimeDeps = d
}
