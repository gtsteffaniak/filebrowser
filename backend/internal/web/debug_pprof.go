//go:build debug

package web

import (
	"net/http"
	_ "net/http/pprof" // registers /debug/pprof handlers on http.DefaultServeMux

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

// startDebugServer exposes pprof on localhost:6060. It is only compiled into
// binaries built with -tags debug, and only started when FILEBROWSER_DEVMODE=true.
func startDebugServer() {
	if !settings.Env.IsDevMode {
		return
	}
	go func() {
		if err := http.ListenAndServe("localhost:6060", nil); err != nil {
			logger.Fatalf("pprof server error: %v", err)
		}
	}()
}
