package activity

import (
	"net"
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/access"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

var (
	config      *settings.Settings
	accessStore *access.Storage
	shareStore  *share.Storage
)

// InitDeps sets package-level dependencies used by activity query and recording.
func InitDeps(cfg *settings.Settings, access *access.Storage, shares *share.Storage) {
	config = cfg
	accessStore = access
	shareStore = shares
}

func remoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	if config != nil {
		xff := r.Header.Get("X-Forwarded-For")
		if config.Http.TrustedHeaders["x-forwarded-for"] && xff != "" {
			ips := strings.Split(xff, ",")
			return strings.TrimSpace(ips[0])
		}
		xri := r.Header.Get("X-Real-IP")
		if config.Http.TrustedHeaders["x-real-ip"] && xri != "" {
			return xri
		}
	}
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	return ip
}
