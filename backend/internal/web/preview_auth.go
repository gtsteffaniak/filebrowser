package web

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Only the private, read-only inline view and media stream routes use this helper.
// A view grant never authenticates mutation, download, metadata, or admin routes.
func withPreviewUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if settings.Config.Auth.Methods.NoAuth {
			return withUserHelper(fn)(w, r, d)
		}
		methods := settings.Config.Auth.Methods
		if methods.ProxyAuth.Enabled && r.Header.Get(methods.ProxyAuth.Header) != "" ||
			methods.JwtAuth.Enabled && (r.Header.Get(methods.JwtAuth.Header) != "" || r.URL.Query().Get("jwt") != "") {
			return withUserHelper(fn)(w, r, d)
		}
		// Explicit credentials retain the normal authentication behavior, including
		// rejecting invalid credentials rather than falling back to a grant.
		_, cookieErr := r.Cookie(sessionCookieName)
		if cookieErr == nil || r.Header.Get("Authorization") != "" || r.URL.Query().Get("auth") != "" ||
			r.Header.Get("Cookie") != "" && strings.Contains(r.Header.Get("Cookie"), sessionCookieName+"=") {
			return withUserHelper(fn)(w, r, d)
		}
		token := r.URL.Query().Get("viewToken")
		if token == "" {
			return withUserHelper(fn)(w, r, d)
		}
		grant, ok := utils.ViewGrantsCache.Get(token)
		if !ok || grant.SessionToken == "" || time.Now().Unix() > grant.ExpiresAt {
			return http.StatusUnauthorized, fmt.Errorf("invalid or expired preview grant")
		}
		if r.Method != http.MethodGet || r.URL.Query().Get("hash") != "" || grant.Source != r.URL.Query().Get("source") {
			return http.StatusForbidden, fmt.Errorf("preview grant scope mismatch")
		}
		// Require a registered web session, not a named API token. The normal
		// middleware below checks its signature, expiry, revocation, and current user.
		_, isSession, registered := state.HashedTokenOwner(grant.SessionToken)
		if !registered || !isSession {
			return http.StatusUnauthorized, fmt.Errorf("preview session is no longer valid")
		}
		authRequest := r.Clone(r.Context())
		authRequest.Header = r.Header.Clone()
		authRequest.Header.Set("Authorization", "Bearer "+grant.SessionToken)
		// Supply the matching cookie internally so normal middleware does not
		// export the stored login JWT through a Set-Cookie response header.
		authRequest.Header.Del("Cookie")
		authRequest.AddCookie(&http.Cookie{Name: sessionCookieName, Value: grant.SessionToken})
		status, err := withUserHelper(nil)(w, authRequest, d)
		if err != nil || status >= 400 {
			return status, err
		}
		if err := ValidateViewGrant(token, d, grant.Source); err != nil {
			return http.StatusForbidden, err
		}
		// Handlers still enforce current per-source permissions, user scopes, and
		// path access rules. Never expose the synthetic Authorization request.
		return fn(w, r, d)
	}
}
