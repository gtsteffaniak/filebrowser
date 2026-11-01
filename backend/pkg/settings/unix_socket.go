package settings

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/user"
	"strconv"
	"strings"
)

type unixPeerCredKey struct{}

// UnixPeerCredential holds the peer process identity for a Unix domain connection.
type UnixPeerCredential struct {
	UID int
	GID int
	Ok  bool
}

// AttachConnContext stores Unix peer credentials on the request context (Linux SO_PEERCRED).
func AttachConnContext(ctx context.Context, c net.Conn) context.Context {
	return attachUnixPeerContext(ctx, c)
}

// UnixPeerFromContext returns peer credentials attached by AttachConnContext.
func UnixPeerFromContext(ctx context.Context) (UnixPeerCredential, bool) {
	cred, ok := ctx.Value(unixPeerCredKey{}).(UnixPeerCredential)
	return cred, ok && cred.Ok
}

// ContextWithUnixPeerForTest attaches synthetic peer credentials (tests only).
func ContextWithUnixPeerForTest(ctx context.Context, uid, gid int) context.Context {
	return context.WithValue(ctx, unixPeerCredKey{}, UnixPeerCredential{UID: uid, GID: gid, Ok: true})
}

func parseSocketFileMode(mode string) (os.FileMode, error) {
	mode = strings.TrimSpace(mode)
	if mode == "" {
		return 0o660, nil
	}
	v, err := strconv.ParseUint(mode, 8, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid http.socketFileMode %q", mode)
	}
	if v&0o777 != v {
		return 0, fmt.Errorf("http.socketFileMode must be a 3-digit octal mode")
	}
	return os.FileMode(v), nil
}

// ListenUnixFilesystem creates a filesystem Unix socket listener with restrictive permissions.
// Abstract sockets (path prefixed with @) skip chmod/chown.
func ListenUnixFilesystem(socketPath string) (net.Listener, error) {
	if strings.HasPrefix(socketPath, "@") {
		return net.Listen("unix", socketPath)
	}
	mode, err := parseSocketFileMode(Config.Http.SocketFileMode)
	if err != nil {
		return nil, err
	}
	var gid int
	setGroup := false
	if group := strings.TrimSpace(Config.Http.SocketGroup); group != "" {
		gid, err = resolveGroupID(group)
		if err != nil {
			return nil, err
		}
		setGroup = true
	}
	oldMask := unixUmask(0o077)
	defer unixUmask(oldMask)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		return nil, err
	}
	closeListener := func() { _ = listener.Close() }
	// Assign group ownership while the socket is still owner-only (umask 077), then grant mode bits.
	if setGroup {
		if err := os.Chown(socketPath, -1, gid); err != nil {
			closeListener()
			return nil, err
		}
	}
	if err := os.Chmod(socketPath, mode); err != nil {
		closeListener()
		return nil, err
	}
	return listener, nil
}

func resolveGroupID(nameOrID string) (int, error) {
	if id, err := strconv.Atoi(nameOrID); err == nil {
		return id, nil
	}
	// Lookup group by name without importing os/user if possible - use user.LookupGroup
	g, err := user.LookupGroup(nameOrID)
	if err != nil {
		return 0, fmt.Errorf("http.socketGroup: %w", err)
	}
	id, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("http.socketGroup: %w", err)
	}
	return id, nil
}

func unixPeerAllowed(uid, gid int) bool {
	uids := Config.Http.UnixSocketTrustedUIDs
	gids := Config.Http.UnixSocketTrustedGIDs
	if len(uids) == 0 && len(gids) == 0 {
		return false
	}
	for _, u := range uids {
		if u == uid {
			return true
		}
	}
	for _, g := range gids {
		if g == gid {
			return true
		}
	}
	return false
}
