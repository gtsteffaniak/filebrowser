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
	Store       *state.Store
	Files       *files.Service
	Auth        *auth.Service
	Transcoding *transcoding.Manager
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

	var transcodeMgr *transcoding.Manager
	if settings.TranscodeEnabled() || settings.Config.Integrations.Media.Transcode.Enabled {
		storeDir, err := transcoding.NewStore(settings.TranscodeCacheDir())
		if err != nil {
			return nil, err
		}
		baseURL := settings.Config.Http.BaseURL
		if settings.Config.Http.ExternalUrl != "" {
			baseURL = settings.Config.Http.ExternalUrl
		}
		transcodeMgr, err = transcoding.NewManager(ctx, transcoding.NewMediaEngine(), storeDir, transcoding.Options{
			BaseURL: strings.TrimRight(baseURL, "/"),
		})
		if err != nil {
			return nil, err
		}
	}

	return &App{
		Store:       store,
		Files:       filesSvc,
		Auth:        authSvc,
		Transcoding: transcodeMgr,
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
