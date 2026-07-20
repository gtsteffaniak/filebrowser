package web

import (
	"net/http"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func normalizeHTTPScheme(proto string) string {
	p := strings.ToLower(strings.TrimSpace(proto))
	switch p {
	case "http", "https":
		return p
	default:
		return ""
	}
}

func firstForwardedValue(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, ","); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return v
}

// requestScheme returns http or https from TLS and optionally trusted X-Forwarded-Proto.
func requestScheme(r *http.Request) string {
	cfg := &settings.Config
	if cfg.Http.TrustedHeaders["x-forwarded-proto"] {
		if proto := normalizeHTTPScheme(r.Header.Get("X-Forwarded-Proto")); proto != "" {
			return proto
		}
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

// requestHost returns the client-facing host, honoring trusted X-Forwarded-Host.
func requestHost(r *http.Request) string {
	cfg := &settings.Config
	if cfg.Http.TrustedHeaders["x-forwarded-host"] {
		if h := firstForwardedValue(r.Header.Get("X-Forwarded-Host")); h != "" {
			return h
		}
	}
	return r.Host
}

// requestSchemeForPublicURL picks scheme for absolute share URLs (HTTPS default when forwarded host is trusted).
func requestSchemeForPublicURL(r *http.Request) string {
	cfg := &settings.Config
	if cfg.Http.TrustedHeaders["x-forwarded-host"] {
		if firstForwardedValue(r.Header.Get("X-Forwarded-Host")) != "" {
			if cfg.Http.TrustedHeaders["x-forwarded-proto"] {
				if proto := normalizeHTTPScheme(r.Header.Get("X-Forwarded-Proto")); proto != "" {
					return proto
				}
			}
			return "https"
		}
	}
	return requestScheme(r)
}

func shareURLParams(r *http.Request) (host, scheme string) {
	if r == nil {
		return "", ""
	}
	return requestHost(r), requestSchemeForPublicURL(r)
}

// ShareURLFromRequest builds a public share or direct-download URL from the request and server config.
func ShareURLFromRequest(r *http.Request, hash string, isDirectDownload bool, token string) string {
	host, scheme := shareURLParams(r)
	return share.PublicShareURL(host, scheme, hash, isDirectDownload, token)
}
