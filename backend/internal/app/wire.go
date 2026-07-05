package app

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
)

// App holds wired dependencies for the running process.
type App struct {
	Store *state.Store
	Files *files.Service
	Auth  *auth.Service
}

// WireServices connects domain packages to the state store after Open.
func WireServices(store *state.Store) (*App, error) {
	filesSvc := files.New(store, store, store)
	authSvc := auth.New(store)
	files.SetDefault(filesSvc)
	auth.SetDefault(authSvc)
	indexing.SetMetaStore(store)
	activity.SetQueryDeps(store, store)
	if err := auth.InitWebAuthn(store); err != nil {
		return nil, err
	}
	return &App{
		Store: store,
		Files: filesSvc,
		Auth:  authSvc,
	}, nil
}

// MustWireServices is for tests; panics on wiring failure.
func MustWireServices(store *state.Store) *App {
	a, err := WireServices(store)
	if err != nil {
		panic(err)
	}
	return a
}
