package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"syscall"
)

// Socket errnos for a client that aborted an in-flight connection. Windows
// reports WSAECONNABORTED (10053); Unix reports EPIPE (32), ECONNABORTED (103)
// or ECONNRESET (104). The values are numeric because the exported syscall
// constants are platform-specific: WSAECONNABORTED only exists on Windows,
// while the POSIX names resolve to invented, never-matching values there.
const (
	errnoBrokenPipe     = syscall.Errno(32)
	errnoConnAborted    = syscall.Errno(103)
	errnoConnReset      = syscall.Errno(104)
	errnoWSAConnAborted = syscall.Errno(10053)
)

// isClientDisconnect reports whether err is the client going away rather than a
// server fault. A browser that navigates away, seeks in a video, or closes the
// tab aborts the TCP connection mid-response; the next write then fails. That is
// normal, so callers should not log it as an error.
//
// It covers the request context already being cancelled, a closed connection,
// and the platform socket errnos for a reset, aborted, or broken-pipe connection.
func isClientDisconnect(r *http.Request, err error) bool {
	if err == nil {
		return false
	}
	if r != nil && r.Context().Err() != nil {
		return true
	}
	if errors.Is(err, context.Canceled) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, os.ErrDeadlineExceeded) {
		return true
	}
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	switch errno {
	case errnoBrokenPipe, errnoConnAborted, errnoConnReset, errnoWSAConnAborted:
		return true
	default:
		return false
	}
}
