package app

import (
	"context"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/activity"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/transcoding"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// App holds wired dependencies for the running process.
type App struct {
	Store *state.Store
	Files *files.Service
	Auth  *auth.Service
}

// WireServices connects domain packages to the state store after Open.
func WireServices(store *state.Store) (*App, error) {
	return WireServicesWithContext(context.Background(), store)
}

// WireServicesWithContext wires services using ctx for background workers.
func WireServicesWithContext(ctx context.Context, store *state.Store) (*App, error) {
	filesSvc := files.New(store, store, store)
	authSvc := auth.New(store)
	files.SetDefault(filesSvc)
	auth.SetDefault(authSvc)
	indexing.SetMetaStore(store)
	activity.SetQueryDeps(store, store)
	if err := auth.InitWebAuthn(store); err != nil {
		return nil, err
	}

	if settings.TranscodeEnabled() {
		storeDir, err := transcoding.NewStore(settings.TranscodeCacheDir())
		if err != nil {
			return nil, err
		}
		// Delivery URLs stay root-relative so a subpath deployment or alternate
		// hostname still resolves and the session cookie is sent.
		transcodeMgr, err := transcoding.NewManager(ctx, transcoding.NewMediaEngine(), storeDir,
			strings.TrimRight(settings.Config.Http.BaseURL, "/"))
		if err != nil {
			return nil, err
		}
		transcoding.SetDefault(transcodeMgr)
	} else {
		transcoding.SetDefault(nil)
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
