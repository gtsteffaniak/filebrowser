package activity

import (
	"net"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func remoteIP(r *http.Request) string {
	cfg := &settings.Config

	xff := r.Header.Get("X-Forwarded-For")
	if cfg.Http.TrustProxyHeaders && xff != "" {
		if ip := settings.TrustedProxyClientIP(xff); ip != "" {
			return ip
		}
	}

	xri := r.Header.Get("X-Real-IP")
	if cfg.Http.TrustProxyHeaders && xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
