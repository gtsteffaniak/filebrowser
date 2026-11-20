//go:build !linux

package settings

import (
	"context"
	"net"
)

func attachUnixPeerContext(ctx context.Context, _ net.Conn) context.Context {
	return ctx
}
