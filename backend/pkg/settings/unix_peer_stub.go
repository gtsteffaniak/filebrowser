//go:build !linux

package settings

import (
	"context"
	"net"
)

func peerCredentialFromConn(net.Conn) (UnixPeerCredential, bool) {
	return UnixPeerCredential{}, false
}

func attachUnixPeerContext(ctx context.Context, _ net.Conn) context.Context {
	return ctx
}
