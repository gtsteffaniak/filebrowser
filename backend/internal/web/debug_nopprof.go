//go:build !debug

package web

// startDebugServer is a no-op in release builds. Build with -tags debug to enable pprof.
func startDebugServer() {}
