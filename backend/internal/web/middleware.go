package web

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/internal/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/internal/auth"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
)

type requestContext = Context

// requestTimeoutError is returned by withTimeout when the handler exceeds its budget.
type requestTimeoutError struct {
	timeout time.Duration
}

func newRequestTimeoutError(timeout time.Duration) error {
	return &requestTimeoutError{timeout: timeout}
}

func (e *requestTimeoutError) Error() string {
	return fmt.Sprintf("request timed out after %s", e.timeout)
}

var FileInfoFasterFunc = func(opts utils.FileOptions, user *users.User) (*iteminfo.ExtendedFileInfo, error) {
	if runtimeDeps.Files != nil {
		return runtimeDeps.Files.FileInfoFaster(opts, user)
	}
	return files.FileInfoFaster(opts, user)
}

type handleFunc = HandleFunc

// Middleware to handle file requests by hash and pass it to the handler
func withHashFileHelper(fn handleFunc) handleFunc {
	return withOrWithoutUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		hash := r.URL.Query().Get("hash")
		inputPath := r.URL.Query().Get("path")
		path, err := utils.SanitizePath(inputPath)
		if err != nil && inputPath != "" {
			return http.StatusBadRequest, err
		}
		path = filepath.ToSlash(path)

		link, err := state.GetShare(hash)
		if err != nil {
			data.Share = share.Share{}
			return http.StatusNotFound, fmt.Errorf("share hash not found")
		}
		// Defensive check: data.User should be set by withOrWithoutUserHelper
		if data.User == nil {
			return http.StatusInternalServerError, fmt.Errorf("internal error: user context not set")
		}
		if link.DisableAnonymous && data.User.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("share is not available to anonymous users")
		}
		// Block anonymous users if per-user download limit is enabled
		if link.PerUserDownloadLimit && data.User.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("anonymous downloads are not allowed with per-user limits")
		}
		if len(link.AllowedUsernames) > 0 {
			if !slices.Contains(link.AllowedUsernames, data.User.Username) {
				return http.StatusForbidden, fmt.Errorf("share is not available to this user")
			}
		}
		data.Share = link
		// Authenticate the share request if needed
		var status int
		if link.Hash != "" {
			status, err = AuthenticateShareRequest(r, link)
			if err != nil || status != http.StatusOK {
				return status, fmt.Errorf("could not authenticate share request")
			}
		}
		source, ok := settings.Config.Server.SourceMap[link.SourcePath]
		if !ok {
			return http.StatusNotFound, fmt.Errorf("source not found")
		}
		if source.Config.Private {
			return http.StatusForbidden, fmt.Errorf("the target source is private")
		}
		// Get file information with options
		getContent := r.URL.Query().Get("content") == "true"
		getMetadata := r.URL.Query().Get("metadata") == "true"
		reachedDownloadsLimit := link.Downloads >= link.DownloadsLimit && link.DownloadsLimit > 0
		if link.DisableFileViewer || reachedDownloadsLimit {
			getContent = false
		}
		userValue, err := state.UserForShareOwner(link)
		if err == nil {
			data.ShareUser = &userValue
		}
		if err != nil {
			return http.StatusNotFound, fmt.Errorf("user for share no longer exists")
		}
		// get user scope path from share
		userScope, err := data.ShareUser.GetScopeForSourceName(source.Name)
		if err != nil {
			return http.StatusForbidden, err
		}
		// so trim user scope from link.Path
		pathWithoutUserScope := utils.JoinPathAsUnix("/", strings.TrimPrefix(link.Path, userScope), path)
		if !strings.HasSuffix(pathWithoutUserScope, "/") {
			pathWithoutUserScope = pathWithoutUserScope + "/"
		}
		data.IndexPath = pathWithoutUserScope
		// skip file fetch for certain apis
		if (r.Method == "POST" && strings.Contains(r.URL.Path, "/resources")) ||
			(r.Method == "GET" && strings.Contains(r.URL.Path, "/resources/items")) ||
			(r.Method == "GET" && strings.Contains(r.URL.Path, "/media/metadata")) ||
			(r.Method == "GET" && strings.Contains(r.URL.Path, "/resources/download")) ||
			(r.Method == "GET" && strings.Contains(r.URL.Path, "/resources/view")) ||
			(r.Method == "GET" && strings.Contains(r.URL.Path, "/media/stream")) {
			return fn(w, r, data)
		}
		file, err := FileInfoFasterFunc(utils.FileOptions{
			Path:                     pathWithoutUserScope,
			Source:                   source.Name,
			Expand:                   true,
			Content:                  getContent,
			Metadata:                 getMetadata,
			AlbumArt:                 strings.Contains(r.URL.Path, "/preview"),
			ExtractEmbeddedSubtitles: settings.Config.Integrations.Media.ExtractEmbeddedSubtitles && link.ExtractEmbeddedSubtitles,
			ShowHidden:               link.ShowHidden,
			HideFileExt:              link.HideFileExt,
			FollowSymlinks:           true,
			ShowPinnedItems:          true,
			ShareHash:                hash,
		}, data.ShareUser)
		if err != nil {
			logger.Errorf("error fetching file info for share. hash=%v path=%v error=%v", hash, path, err)
			return ErrToStatus(err), fmt.Errorf("error fetching share from server")
		}
		file.Token = link.Token
		file.Source = link.Hash
		file.Hash = link.Hash
		if !link.EnableOnlyOffice || link.DisableFileViewer || reachedDownloadsLimit {
			file.OnlyOfficeId = ""
		}
		if file.Type != "directory" {
			AttachViewToken(data, source.Name, path, file)
		} else {
			AttachViewTokensForDirectory(data, source.Name, path, file)
		}
		file.Path = utils.AddTrailingSlashIfNotExists(path)
		// Set the file info in the `data` object
		data.FileInfo = *file
		// Call the next handler with the data
		return fn(w, r, data)
	})
}

// Middleware to ensure the user is an admin
func withAdminHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// Ensure the user has admin permissions
		if !data.User.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		return fn(w, r, data)
	})
}

// extractUserFromExpiredToken attempts to extract user information from an expired token
// This is used by withOrWithoutUserHelper to get user context even when tokens are expired
func extractUserFromExpiredToken(r *http.Request, data *requestContext) *users.User {
	if settings.Config.Auth.Methods.NoAuth {
		admin := settings.Config.Auth.AdminUsername
		if admin == "" {
			admin = "admin"
		}
		userValue, err := state.GetUserByUsername(admin)
		var user *users.User
		if err == nil {
			user = &userValue
		}
		if err != nil {
			logger.Errorf("no auth: %v", err)
			return nil
		}
		return user
	}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(settings.Config.Auth.Key), nil
	}

	tokenString, err := ExtractToken(r)
	if err != nil {
		return nil
	}

	data.Token = tokenString
	var tk users.AuthToken
	token, err := jwt.ParseWithClaims(tokenString, &tk, keyFunc)
	if err != nil {
		return nil
	}

	if !token.Valid {
		return nil
	}

	// Token is valid (but might be expired or revoked)
	// Try to get the user regardless of expiration status
	var user *users.User
	userValue, err := state.UserFromAPIToken(tk, tokenString)
	if err == nil {
		user = &userValue
	}
	if err != nil {
		logger.Errorf("Failed to get user from token: %v", err)
		return nil
	}

	if user.Username == "" {
		return nil
	}

	return user
}

// withOrWithoutUserHelper is a middleware that tries to authenticate a user.
// If authentication is successful, the user is added to the request context.
// If authentication fails, the request continues without a user.
func withOrWithoutUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		var snap share.Share
		var haveSnap bool
		var isShareRequest bool
		var shareHash string

		hash := r.URL.Query().Get("hash")
		if hash != "" {
			isShareRequest = true
			shareHash = hash
			if l, err := state.GetShare(hash); err == nil {
				snap = l
				haveSnap = true
			}
		} else {
			prefix := settings.Config.Server.BaseURL + "public/share/"
			reconstructed := settings.Config.Server.BaseURL + "public" + r.URL.Path
			if strings.HasPrefix(reconstructed, prefix) {
				remaining := strings.TrimPrefix(reconstructed, prefix)
				if remaining != "" {
					if idx := strings.IndexByte(remaining, '/'); idx >= 0 {
						remaining = remaining[:idx]
					}
					if remaining != "" {
						isShareRequest = true
						shareHash = remaining
						if l, err := state.GetShare(remaining); err == nil {
							snap = l
							haveSnap = true
						} else {
							logger.Debugf("error getting share by hash: %v", err)
						}
					}
				}
			}
		}

		if isShareRequest {
			if haveSnap {
				data.Share = snap
				data.ShareValid = true
			} else {
				data.Share = share.Share{ShareColumns: share.ShareColumns{Hash: shareHash}}
				data.ShareValid = false
			}
		}

		// Try to authenticate user first
		status, err := withUserHelper(nil)(w, r, data)
		if err == nil && status < 400 {
			if data.ShareValid && data.Share.Hash != "" {
				if data.User != nil {
					data.User.CustomTheme = data.Share.ShareTheme
				}
			}
			return fn(w, r, data)
		}

		// Authentication failed, but try to extract user info from expired tokens
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			// Try to extract user info from potentially expired token
			userFromExpiredToken := extractUserFromExpiredToken(r, data)
			if userFromExpiredToken != nil {
				data.User = userFromExpiredToken
				if data.ShareValid && data.Share.Hash != "" {
					data.User.CustomTheme = data.Share.ShareTheme
				}
				SetUserInResponseWriter(w, data.User)
				return fn(w, r, data)
			}

			// No valid token or user found, fall back to anonymous
			data.User = &users.User{
				FrontendUser: users.FrontendUser{Username: "anonymous"},
			}
			state.ApplyUserDefaults(data.User)
			// Clear any user data that might have been partially set
			data.Token = ""
			if data.ShareValid && data.Share.Hash != "" {
				data.User.CustomTheme = data.Share.ShareTheme
			}
			// Call the handler function without user context
			return fn(w, r, data)
		}
		return status, fmt.Errorf("could not authenticate request")
	}
}

func withoutUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// This middleware is used when no user authentication is required
		// Call the actual handler function with the updated context
		return fn(w, r, data)
	}
}

// allow user without OTP to pass
func LoginHelper(disableOtp bool, fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if settings.Config.Auth.Methods.ProxyAuth.Enabled {
			proxyUser := r.Header.Get(settings.Config.Auth.Methods.ProxyAuth.Header)
			if proxyUser != "" {
				return getProxyUser(w, r, d, fn, proxyUser)
			}
		}
		// Check if request has a valid admin token first
		if tokenStr, err := ExtractToken(r); err == nil && tokenStr != "" {
			keyFunc := func(token *jwt.Token) (interface{}, error) {
				return []byte(settings.Config.Auth.Key), nil
			}
			var tk users.AuthToken
			if token, err := jwt.ParseWithClaims(tokenStr, &tk, keyFunc); err == nil && token.Valid {
				if !state.IsTokenRevoked( tokenStr) {
					userValue, err := state.UserFromAPIToken(tk, tokenStr)
					if err == nil && userValue.Permissions.Admin {
						u := userValue
						d.User = &u
						return fn(w, r, d)
					}
				}
			}
		}

		// Try LDAP first if enabled; on success set d.User and continue to handler
		if settings.Config.Auth.Methods.LdapAuth.Enabled {
			// No valid admin token - proceed with username/password authentication
			username := r.URL.Query().Get("username")
			password := r.Header.Get("X-Password")
			// URL-decode password to support special characters in headers
			password, err := url.QueryUnescape(password)
			if err != nil {
				return 401, fmt.Errorf("invalid password encoding")
			}
			logger.Debug("ldap auth, calling AuthenticateLDAPUser")
			ldapUser, err := AuthenticateLDAPUser(username, password)
			if err == nil {
				logger.Debugf("ldap auth successful, calling handler")
				d.User = ldapUser
				return fn(w, r, d)
			}
			logger.Debug("ldap auth failed, calling password auth", err)
		}
		if settings.Config.Auth.Methods.PasswordAuth.Enabled {
			user, err := auth.AuthenticatePassword(r, true)
			if err != nil {
				logger.Debug("password auth failed, calling handler:", err)
				if err == errors.ErrNoTotpProvided {
					return 403, err
				}
				return 401, errors.ErrUnauthorized
			}
			d.User = user
			return fn(w, r, d)
		}
		return withUserHelper(fn)(w, r, d)
	}
}

// Middleware to retrieve and authenticate user
func withUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		if settings.Config.Auth.Methods.NoAuth {
			admin := settings.Config.Auth.AdminUsername
			if admin == "" {
				admin = "admin"
			}
			userValue, err := state.GetUserByUsername(admin)
			if err == nil {
				data.User = &userValue
			}
			if err != nil {
				logger.Errorf("no auth: %v", err)
				return http.StatusInternalServerError, err
			}
			if fn == nil {
				return http.StatusOK, nil
			}
			return fn(w, r, data)
		}

		// Check for JWT external auth first (header or query param)
		if settings.Config.Auth.Methods.JwtAuth.Enabled {
			jwtToken := r.Header.Get(settings.Config.Auth.Methods.JwtAuth.Header)
			if jwtToken == "" {
				// Check query parameter (hardcoded to "jwt")
				jwtToken = r.URL.Query().Get("jwt")
			}

			if jwtToken != "" {
				return getJwtUser(w, r, data, fn, jwtToken)
			}
		}

		proxyUser := r.Header.Get(settings.Config.Auth.Methods.ProxyAuth.Header)
		isProxyUser := settings.Config.Auth.Methods.ProxyAuth.Enabled && proxyUser != ""
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return []byte(settings.Config.Auth.Key), nil
		}
		if data.Token == "" {
			var err error
			data.Token, err = ExtractToken(r)
			if err != nil && !isProxyUser {
				return http.StatusUnauthorized, err
			}
		}

		var tk users.AuthToken
		token, err := jwt.ParseWithClaims(data.Token, &tk, keyFunc)
		if err != nil {
			if isProxyUser {
				return getProxyUser(w, r, data, fn, proxyUser)
			}
			// JWT library automatically validates expiration - if expired, it returns an error
			return http.StatusUnauthorized, fmt.Errorf("invalid token: %v", err)
		}
		if !token.Valid {
			return http.StatusUnauthorized, fmt.Errorf("invalid token")
		}
		if state.IsTokenRevoked( data.Token) {
			return http.StatusUnauthorized, fmt.Errorf("token is expired or revoked")
		}
		// ExpiresAt should always be set in valid tokens created by our system
		// JWT library populates RegisteredClaims.ExpiresAt
		if tk.RegisteredClaims.ExpiresAt == nil {
			return http.StatusUnauthorized, fmt.Errorf("token is invalid or revoked")
		}
		// Check if token is about to expire for renewal header
		if tk.RegisteredClaims.ExpiresAt.Unix() < time.Now().Add(time.Minute*30).Unix() {
			w.Header().Add("X-Renew-Token", "true")
		}
		userValue, err := state.UserFromAPIToken(tk, data.Token)
		if err == nil {
			data.User = &userValue
		}
		if err != nil {
			logger.Errorf("Failed to get user from token: %v", err)
			return http.StatusUnauthorized, fmt.Errorf("token is invalid or revoked")
		}
		if tokenName, ok := state.TokenNameForRawToken(data.User, data.Token); ok {
			applyNamedApiTokenGlobalCaps(data.User, tk, tokenName)
		}

		// Set cookie. Some clients like gvfs relies on it for concurrent uploads
		if tk.RegisteredClaims.ExpiresAt != nil {
			SetSessionCookie(w, r, data.Token, tk.RegisteredClaims.ExpiresAt.Time)
		}
		SetUserInResponseWriter(w, data.User)
		if data.User.Username == "" {
			return http.StatusForbidden, errors.ErrUnauthorized
		}
		// Call the handler function, passing in the context (or return OK if no handler)
		if fn == nil {
			return http.StatusOK, nil
		}
		return fn(w, r, data)
	}
}

func getJwtUser(w http.ResponseWriter, r *http.Request, data *requestContext, fn handleFunc, jwtToken string) (int, error) {
	// Verify the external JWT token
	username, claims, err := auth.VerifyExternalJWT(
		jwtToken,
		settings.Config.Auth.Methods.JwtAuth.Secret,
		settings.Config.Auth.Methods.JwtAuth.Algorithm,
		settings.Config.Auth.Methods.JwtAuth.UserIdentifier,
	)
	if err != nil {
		logger.Debugf("JWT verification failed: %v", err)
		return http.StatusForbidden, fmt.Errorf("JWT authentication failed: %w", err)
	}

	// Setup user based on JWT claims
	user, err := SetupJwtUser(r, data, username, claims)
	if err != nil {
		return http.StatusForbidden, err
	}
	data.User = user
	SetUserInResponseWriter(w, data.User)
	if data.User.Username == "" {
		return http.StatusForbidden, errors.ErrUnauthorized
	}

	// Generate a FileBrowser session token for JWT users if they don't have one
	if data.Token == "" {
		expires := time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)
		tokenString, _, err := auth.MakeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), expires, user.Permissions, false)
		if err != nil {
			logger.Errorf("Failed to generate token for JWT user %s: %v", username, err)
			return http.StatusInternalServerError, fmt.Errorf("failed to generate token")
		}
		data.Token = tokenString
		SetSessionCookie(w, r, tokenString, time.Now().Add(expires).Add(time.Minute*30))
	}

	// Call the handler function, passing in the context (or return OK if no handler)
	if fn == nil {
		return http.StatusOK, nil
	}
	return fn(w, r, data)
}

func getProxyUser(w http.ResponseWriter, r *http.Request, data *requestContext, fn handleFunc, proxyUser string) (int, error) {
	// proxy user logic
	user, err := SetupProxyUser(r, data, proxyUser)
	if err != nil {
		return http.StatusForbidden, err
	}
	data.User = user
	SetUserInResponseWriter(w, data.User)
	if data.User.Username == "" {
		return http.StatusForbidden, errors.ErrUnauthorized
	}
	// Generate a token for proxy users if they don't have one
	if data.Token == "" {
		expires := time.Hour * time.Duration(settings.Config.Auth.TokenExpirationHours)
		tokenString, _, err := auth.MakeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), expires, user.Permissions, false)
		if err != nil {
			logger.Errorf("Failed to generate token for proxy user %s: %v", proxyUser, err)
			return http.StatusInternalServerError, fmt.Errorf("failed to generate token")
		}
		data.Token = tokenString
		SetSessionCookie(w, r, tokenString, time.Now().Add(expires).Add(time.Minute*30))
	}
	// Call the handler function, passing in the context (or return OK if no handler)
	if fn == nil {
		return http.StatusOK, nil
	}
	return fn(w, r, data)
}

// Middleware to ensure the user is either the requested user or an admin
func withSelfOrAdminHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// Check if the current user is the same as the requested user or if they are an admin
		if !data.User.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		// Call the actual handler function with the updated context
		return fn(w, r, data)
	})
}

func wrapHandler(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &requestContext{
			Ctx: r.Context(),
		}

		// Call the actual handler function and get status code and error
		status, err := fn(w, r, data)
		// Handle the error case if there is one
		if err != nil {
			// Create an error response in JSON format
			response := &HttpResponse{
				Status:  status, // Use the status code from the middleware
				Message: err.Error(),
			}

			// Set the content type to JSON and status code
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(status)

			// Marshal the error response to JSON
			errorBytes, marshalErr := json.Marshal(response)
			if marshalErr != nil {
				logger.Errorf("Error marshalling error response: %v", marshalErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Write the JSON error response
			if _, writeErr := w.Write(errorBytes); writeErr != nil {
				logger.Debugf("Error writing error response: %v", writeErr)
			}
			return
		}

		// No error, proceed to write status if non-zero
		if status != 0 {
			w.WriteHeader(status)
		}
	}
}

// wrapHandlerBasicAuth wraps a handler and automatically sets WWW-Authenticate header
// for 401 Unauthorized responses, triggering Basic Auth challenge
func wrapHandlerBasicAuth(fn handleFunc) http.HandlerFunc {
	// Wrap the handler to set WWW-Authenticate header for 401 responses
	wrappedFn := func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		status, err := fn(w, r, data)
		// Set WWW-Authenticate header before returning 401
		if status == http.StatusUnauthorized {
			w.Header().Set("WWW-Authenticate", `Basic realm="WebDAV"`)
		}
		return status, err
	}
	return wrapHandler(wrappedFn)
}

func withPermShareHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !d.User.Permissions.Share {
			return http.StatusForbidden, nil
		}
		return fn(w, r, d)
	})
}

// withBasicAuthHelper extracts Basic Auth credentials and uses the password as a JWT token
// to authenticate the user. The username is ignored, and the password should be a JWT token.
func withBasicAuthHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		_, password, ok := r.BasicAuth()
		if !ok || password == "" {
			// Return 401 - wrapHandlerBasicAuth will set WWW-Authenticate header
			return http.StatusUnauthorized, fmt.Errorf("basic authentication required")
		}
		data.Token = password
		return withUserHelper(fn)(w, r, data)
	}
}

// withBasicAuth returns an http.HandlerFunc for use with router.Handle.
// It extracts Basic Auth credentials and uses the password as a JWT token to authenticate the user.
func withBasicAuth(fn handleFunc) http.HandlerFunc {
	return wrapHandlerBasicAuth(withBasicAuthHelper(fn))
}

func withPermShare(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withPermShareHelper(fn))
}

func withHashFile(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withHashFileHelper(fn))
}

func withAdmin(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withAdminHelper(fn))
}

func withUser(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withUserHelper(fn))
}

func withOrWithoutUser(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withOrWithoutUserHelper(fn))
}

func withoutUser(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withoutUserHelper(fn))
}

func loginHelper(fn handleFunc) handleFunc {
	return LoginHelper(false, fn)
}

func withSelfOrAdmin(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withSelfOrAdminHelper(fn))
}

// withTimeoutHelper adds a configurable timeout context to any operation
func withTimeoutHelper(timeout time.Duration, fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// Create a context with the specified timeout
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		// Log timeout warning at 80% of timeout duration
		warningTime := time.Duration(float64(timeout) * 0.8)
		go func() {
			select {
			case <-time.After(warningTime):
				if ctx.Err() == nil {
					logger.Api(http.StatusRequestTimeout, fmt.Sprintf("Request approaching timeout (%.1fs/%.0fs): %s %s", warningTime.Seconds(), timeout.Seconds(), r.Method, r.URL.Path))
				}
			case <-ctx.Done():
				// Context finished before warning time
				return
			}
		}()

		// Replace the request context with the timeout context
		r = r.WithContext(ctx)
		data.Ctx = ctx
		// Call the handler and check for timeout
		status, err := fn(w, r, data)

		if ctx.Err() == context.DeadlineExceeded {
			return http.StatusRequestTimeout, newRequestTimeoutError(timeout)
		}

		return status, err
	}
}

func withTimeout(timeout time.Duration, fn handleFunc) http.HandlerFunc {
	return wrapHandler(withTimeoutHelper(timeout, fn))
}

func muxWithMiddleware(mux *http.ServeMux) *http.ServeMux {
	wrappedMux := http.NewServeMux()
	wrappedMux.Handle("/", LoggingMiddleware(mux))
	return wrappedMux
}
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DEFER RECOVERY FUNCTION
		defer func() {
			if rcv := recover(); rcv != nil {
				method := r.Method
				url := r.URL.String()
				username := "unknown" // Default username

				// Attempt to get username from ResponseWriterWrapper if it's set
				if ww, ok := w.(*ResponseWriterWrapper); ok && ww.User != "" {
					username = ww.User
				}
				// Get Go-level stack trace
				buf := make([]byte, 16384)     // Increased buffer size for potentially long CGo traces
				n := runtime.Stack(buf, false) // false for current goroutine only
				stackTrace := string(buf[:n])

				logger.Errorf("PANIC RECOVERED: %v\nUser: %s\nMethod: %s\nURL: %s\nRemoteAddr: %s\nGo Stack Trace:\n%s",
					rcv, username, method, url, GetRemoteIP(r), stackTrace)

				// Attempt to send a 500 error response to the client
				// This is a best-effort; the connection might be broken or process too unstable.
				if ww, ok := w.(*ResponseWriterWrapper); ok {
					if !ww.WroteHeader {
						ww.Header().Set("Content-Type", "application/json; charset=utf-8")
						ww.WriteHeader(http.StatusInternalServerError)
					}
				} else {
					_, _ = RenderJSON(w, r, &HttpResponse{
						Status:  500,
						Message: "A critical internal error occurred. Please try again later.",
					}, http.StatusInternalServerError)
				}

			}
		}()

		start := time.Now()
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler in the chain
		next.ServeHTTP(wrappedWriter, r)

		// Existing logging logic for normal requests
		fullURL := r.URL.Path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}
		truncUser := wrappedWriter.User
		if truncUser == "" {
			truncUser = "N/A" // Handle case where user might not be set (e.g., if panic occurred before user auth)
		} else if len(truncUser) > 12 {
			truncUser = truncUser[:10] + ".."
		}
		duration := time.Since(start)

		// ApiPathExclude is applied per logging sink inside go-logger (logger.ApiPath).
		logger.ApiPath(wrappedWriter.StatusCode, fullURL,
			fmt.Sprintf("%-7s | %3d | %-15s | %-12s | %-12s | \"%s\"",
				r.Method,
				wrappedWriter.StatusCode,
				GetRemoteIP(r),
				truncUser,
				fmt.Sprintf("%vms", duration.Milliseconds()),
				fullURL))
	})
}
