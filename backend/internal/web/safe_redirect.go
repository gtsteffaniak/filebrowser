package web

import (
	"net/url"
	"strings"
)

// SafeRelativeRedirectPath reports whether raw is a safe in-app path for post-login redirects.
// Only single-leading-slash paths are allowed; protocol-relative and backslash tricks are rejected.
func SafeRelativeRedirectPath(raw string) (string, bool) {
	if raw == "" {
		return "", false
	}
	if strings.ContainsAny(raw, "\r\n\x00") {
		return "", false
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		decoded = raw
	}
	if strings.ContainsAny(decoded, "\r\n\x00") {
		return "", false
	}
	decoded = strings.TrimSpace(decoded)
	if decoded == "" {
		return "", false
	}
	if strings.Contains(decoded, "://") {
		return "", false
	}
	if !strings.HasPrefix(decoded, "/") {
		return "", false
	}
	if strings.HasPrefix(decoded, "//") {
		return "", false
	}
	if strings.HasPrefix(decoded, "/\\") {
		return "", false
	}
	return decoded, true
}
