package activity

import (
	"net"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func remoteIP(r *http.Request) string {
	cfg := &settings.Config

	xff := r.Header.Get("X-Forwarded-For")
	if cfg.Http.TrustedHeaders["x-forwarded-for"] && xff != "" {
		ips := strings.Split(xff, ",")
		return strings.TrimSpace(ips[0])
	}

	xri := r.Header.Get("X-Real-IP")
	if cfg.Http.TrustedHeaders["x-real-ip"] && xri != "" {
		return xri
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
