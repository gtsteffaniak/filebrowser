//go:build linux

package settings

import (
	"context"
	"net"

	"golang.org/x/sys/unix"
)

func peerCredentialFromConn(c net.Conn) (UnixPeerCredential, bool) {
	uc, ok := c.(*net.UnixConn)
	if !ok {
		return UnixPeerCredential{}, false
	}
	raw, err := uc.SyscallConn()
	if err != nil {
		return UnixPeerCredential{}, false
	}
	var ucred *unix.Ucred
	var ctlErr error
	ctlErr = raw.Control(func(fd uintptr) {
		ucred, ctlErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	})
	if ctlErr != nil || ucred == nil {
		return UnixPeerCredential{}, false
	}
	return UnixPeerCredential{UID: int(ucred.Uid), GID: int(ucred.Gid), Ok: true}, true
}

func attachUnixPeerContext(ctx context.Context, c net.Conn) context.Context {
	if cred, ok := peerCredentialFromConn(c); ok {
		ctx = context.WithValue(ctx, unixPeerCredKey{}, cred)
	}
	return ctx
}
