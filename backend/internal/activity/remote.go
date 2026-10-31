package activity

import (
	"net"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func remoteIP(r *http.Request) string {
	cfg := &settings.Config

	// Forwarded client headers are only honored when the socket peer is a
	// configured trusted proxy; otherwise a direct client could spoof its IP.
	trustedPeer := cfg.Http.TrustProxyHeaders && settings.IsTrustedProxyPeer(r)

	xff := r.Header.Get("X-Forwarded-For")
	if trustedPeer && xff != "" {
		if ip := settings.TrustedProxyClientIP(xff); ip != "" {
			return ip
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if trustedPeer && xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
