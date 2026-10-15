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

	// Create the manager whenever ffmpeg is available so enabling transcoding at
	// runtime (admin media settings) takes effect without a restart.
	var transcodeMgr *transcoding.Manager
	if settings.MediaEnabled() {
		storeDir, err := transcoding.NewStore(settings.TranscodeCacheDir())
		if err != nil {
			return nil, err
		}
		// Root-relative delivery URLs only: the playlist and its segments are
		// fetched from the same origin that served the page, so a subpath
		// deployment or an alternate hostname (LAN IP, internal DNS) still
		// resolves, and the session cookie is sent. http.externalUrl is stripped
		// of the base path by setupUrls and would produce 404/401 URLs here.
		transcodeMgr, err = transcoding.NewManager(ctx, transcoding.NewMediaEngine(), storeDir, transcoding.Options{
			BaseURL: strings.TrimRight(settings.Config.Http.BaseURL, "/"),
		})
		if err != nil {
			return nil, err
		}
	}
	// Register the manager (nil when ffmpeg is unavailable) so the web layer can
	// reach it without it being threaded through StartHttp's Deps.
	transcoding.SetDefault(transcodeMgr)

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
