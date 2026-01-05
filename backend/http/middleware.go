package http

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"runtime"
	"slices"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

type requestContext struct {
	user         *users.User
	fileInfo     iteminfo.ExtendedFileInfo
	token        string
	share        *share.Link
	shareValid   bool
	ctx          context.Context
	MaxBandwidth int
}

type HttpResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

var FileInfoFasterFunc = files.FileInfoFaster

// Updated handleFunc to match the new signature
type handleFunc func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error)

// Middleware to handle file requests by hash and pass it to the handler
func withHashFileHelper(fn handleFunc) handleFunc {
	return withOrWithoutUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		hash := r.URL.Query().Get("hash")
		encodedPath := r.URL.Query().Get("path")
		// Decode the URL-encoded path - use PathUnescape to preserve + as literal character
		path, err := url.PathUnescape(encodedPath)
		if err != nil {
			return http.StatusBadRequest, fmt.Errorf("invalid path encoding: %v", err)
		}

		// Get the file link by hash
		link, err := store.Share.GetByHash(hash)
		if err != nil {
			data.share = &share.Link{}
			return http.StatusNotFound, fmt.Errorf("share hash not found")
		}
		if link.DisableAnonymous && data.user.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("share is not available to anonymous users")
		}
		// Block anonymous users if per-user download limit is enabled
		if link.PerUserDownloadLimit && data.user.Username == "anonymous" {
			return http.StatusForbidden, fmt.Errorf("anonymous downloads are not allowed with per-user limits")
		}
		if len(link.AllowedUsernames) > 0 {
			if !slices.Contains(link.AllowedUsernames, data.user.Username) {
				return http.StatusForbidden, fmt.Errorf("share is not available to this user")
			}
		}
		// Check per-user download limit
		if link.PerUserDownloadLimit && link.HasReachedUserLimit(data.user.Username) {
			return http.StatusForbidden, fmt.Errorf("user download limit reached for this share")
		}
		data.share = link
		// Authenticate the share request if needed
		var status int
		if link.Hash != "" {
			status, err = authenticateShareRequest(r, link)
			if err != nil || status != http.StatusOK {
				return status, fmt.Errorf("could not authenticate share request")
			}
		}
		source, ok := config.Server.SourceMap[link.Source]
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
		shareCreatedByUser, err := store.Users.Get(link.UserID)
		if err != nil {
			return http.StatusNotFound, fmt.Errorf("user for share no longer exists")
		}
		file, err := FileInfoFasterFunc(utils.FileOptions{
			Path:                     utils.JoinPathAsUnix(link.Path, path),
			Source:                   link.Source,
			Username:                 shareCreatedByUser.Username,
			Expand:                   true,
			Content:                  getContent,
			Metadata:                 getMetadata,
			ExtractEmbeddedSubtitles: settings.Config.Integrations.Media.ExtractEmbeddedSubtitles && link.ExtractEmbeddedSubtitles,
		}, store.Access)
		if err != nil {
			logger.Errorf("error fetching file info for share. hash=%v path=%v error=%v", hash, path, err)
			return errToStatus(err), fmt.Errorf("error fetching share from server")
		}
		file.Token = link.Token
		file.Source = ""
		file.Hash = link.Hash
		if !link.EnableOnlyOffice || link.DisableFileViewer || reachedDownloadsLimit {
			file.OnlyOfficeId = ""
		}
		if getContent && file.Content != "" {
			link.Mu.Lock()
			link.Downloads++
			link.Mu.Unlock()
			// Track per-user download if enabled
			if link.PerUserDownloadLimit {
				link.IncrementUserDownload(data.user.Username)
			}
		}
		file.Path = utils.AddTrailingSlashIfNotExists(path)
		// Set the file info in the `data` object
		data.fileInfo = *file
		// Call the next handler with the data
		return fn(w, r, data)
	})
}

// Middleware to ensure the user is an admin
func withAdminHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// Ensure the user has admin permissions
		if !data.user.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		return fn(w, r, data)
	})
}

// extractUserFromExpiredToken attempts to extract user information from an expired token
// This is used by withOrWithoutUserHelper to get user context even when tokens are expired
func extractUserFromExpiredToken(r *http.Request, data *requestContext) *users.User {
	if config.Auth.Methods.NoAuth {
		user, err := store.Users.Get(uint(1))
		if err != nil {
			logger.Errorf("no auth: %v", err)
			return nil
		}
		return user
	}

	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(config.Auth.Key), nil
	}

	tokenString, err := extractToken(r)
	if err != nil {
		return nil
	}

	data.token = tokenString
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
	user, err := store.Users.Get(tk.BelongsTo)
	if err != nil {
		logger.Errorf("Failed to get user with ID %v: %v", tk.BelongsTo, err)
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
		var link *share.Link
		var isShareRequest bool
		var shareHash string

		hash := r.URL.Query().Get("hash")
		if hash != "" {
			// Get the file link by hash
			isShareRequest = true
			shareHash = hash
			link, _ = store.Share.GetByHash(hash)
		} else {
			prefix := config.Server.BaseURL + "public/share/"
			reconstructed := config.Server.BaseURL + "public" + r.URL.Path
			if strings.HasPrefix(reconstructed, prefix) {
				remaining := strings.TrimPrefix(reconstructed, prefix)
				if remaining != "" {
					if idx := strings.IndexByte(remaining, '/'); idx >= 0 {
						remaining = remaining[:idx]
					}
					if remaining != "" {
						isShareRequest = true
						shareHash = remaining
						var err error
						link, err = store.Share.GetByHash(remaining)
						if err != nil {
							logger.Debugf("error getting share by hash: %v", err)
						}
					}
				}
			}
		}

		// If this is a share request, always create a share context (even if invalid)
		if isShareRequest {
			if link != nil {
				data.share = link
				data.shareValid = true
			} else {
				// Create an empty share with just the hash for invalid shares
				data.share = &share.Link{Hash: shareHash}
				data.shareValid = false
			}
		}

		// Try to authenticate user first
		status, err := withUserHelper(nil)(w, r, data)
		if err == nil && status < 400 {
			if data.share != nil && data.shareValid {
				if data.user != nil {
					data.user.CustomTheme = data.share.ShareTheme
				}
			}
			return fn(w, r, data)
		}

		// Authentication failed, but try to extract user info from expired tokens
		if status == http.StatusUnauthorized || status == http.StatusForbidden {
			// Try to extract user info from potentially expired token
			userFromExpiredToken := extractUserFromExpiredToken(r, data)
			if userFromExpiredToken != nil {
				data.user = userFromExpiredToken
				if data.share != nil && data.shareValid {
					data.user.CustomTheme = data.share.ShareTheme
				}
				setUserInResponseWriter(w, data.user)
				return fn(w, r, data)
			}

			// No valid token or user found, fall back to anonymous
			data.user = &users.User{Username: "anonymous"}
			settings.ApplyUserDefaults(data.user)
			// Clear any user data that might have been partially set
			data.token = ""
			if data.share != nil && data.shareValid {
				data.user.CustomTheme = data.share.ShareTheme
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
func userWithoutOTPhelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		// This middleware is used when no user authentication is required
		// Call the actual handler function with the updated context
		username := r.URL.Query().Get("username")
		password := r.Header.Get("X-Password")
		if username == "" || password == "" {
			return withUserHelper(fn)(w, r, d)
		} else {
			if !config.Auth.Methods.PasswordAuth.Enabled {
				return 401, errors.ErrUnauthorized
			}
			// Get the authentication method from the settings
			auther, err := store.Auth.Get("password")
			if err != nil {
				return 401, errors.ErrUnauthorized
			}
			// Authenticate the user based on the request
			user, err := auther.Auth(r, store.Users)
			if err != nil {
				if err == errors.ErrNoTotpProvided {
					return 403, err
				}
				return 401, errors.ErrUnauthorized
			}
			d.user = user
		}
		return fn(w, r, d)
	}
}

// Middleware to retrieve and authenticate user
func withUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		if config.Auth.Methods.NoAuth {
			var err error
			// Retrieve the user from the store and store it in the context
			data.user, err = store.Users.Get(uint(1))
			if err != nil {
				logger.Errorf("no auth: %v", err)
				return http.StatusInternalServerError, err
			}
			if fn == nil {
				return http.StatusOK, nil
			}
			return fn(w, r, data)
		}
		proxyUser := r.Header.Get(config.Auth.Methods.ProxyAuth.Header)
		isProxyUser := config.Auth.Methods.ProxyAuth.Enabled && proxyUser != ""
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Auth.Key), nil
		}
		tokenString, err := extractToken(r)
		if err != nil && !isProxyUser {
			return http.StatusUnauthorized, err
		}
		data.token = tokenString
		var tk users.AuthToken
		token, err := jwt.ParseWithClaims(tokenString, &tk, keyFunc)
		if err != nil {
			if isProxyUser {
				return getProxyUser(w, r, data, fn, proxyUser)
			}
			// JWT library automatically validates expiration - if expired, it returns an error
			return http.StatusUnauthorized, fmt.Errorf("error processing token, %v", err)
		}
		if !token.Valid {
			return http.StatusUnauthorized, fmt.Errorf("invalid token")
		}
		if auth.IsRevokedApiKey(data.token) {
			if isProxyUser {
				return getProxyUser(w, r, data, fn, proxyUser)
			}
			return http.StatusUnauthorized, fmt.Errorf("token revoked")
		}
		// ExpiresAt should always be set in valid tokens created by our system
		// If it's nil, the token is invalid
		if tk.ExpiresAt == nil {
			return http.StatusUnauthorized, fmt.Errorf("invalid token: missing expiration")
		}
		// Check if token is about to expire for renewal header
		if tk.ExpiresAt.Unix() < time.Now().Add(time.Minute*30).Unix() {
			w.Header().Add("X-Renew-Token", "true")
		}
		// Check if token is minimal/stateful (no BelongsTo in claim)
		if tk.BelongsTo == 0 {
			tk.BelongsTo, err = getUserFromApiToken(data.token)
			if err != nil {
				return http.StatusUnauthorized, err
			}
		}
		data.user, err = store.Users.Get(tk.BelongsTo)
		if err != nil {
			logger.Errorf("Failed to get user with ID %v: %v", tk.BelongsTo, err)
			return http.StatusInternalServerError, err
		}
		// Set cookie. Some clients like gvfs relies on it for concurrent uploads
		if expire := tk.ExpiresAt; expire != nil {
			setSessionCookie(w, r, tokenString, expire.Time)
		}
		setUserInResponseWriter(w, data.user)
		if data.user.Username == "" {
			return http.StatusForbidden, errors.ErrUnauthorized
		}
		// Call the handler function, passing in the context (or return OK if no handler)
		if fn == nil {
			return http.StatusOK, nil
		}
		return fn(w, r, data)
	}
}

func getProxyUser(w http.ResponseWriter, r *http.Request, data *requestContext, fn handleFunc, proxyUser string) (int, error) {
	// proxy user logic
	user, err := setupProxyUser(r, data, proxyUser)
	if err != nil {
		return http.StatusForbidden, err
	}
	data.user = user
	setUserInResponseWriter(w, data.user)
	if data.user.Username == "" {
		return http.StatusForbidden, errors.ErrUnauthorized
	}
	// Generate a token for proxy users if they don't have one
	if data.token == "" {
		expires := time.Hour * time.Duration(config.Auth.TokenExpirationHours)
		signed, err := makeSignedTokenAPI(user, "WEB_TOKEN_"+utils.InsecureRandomIdentifier(4), expires, user.Permissions, false)
		if err != nil {
			logger.Errorf("Failed to generate token for proxy user %s: %v", proxyUser, err)
			return http.StatusInternalServerError, fmt.Errorf("failed to generate token")
		}
		data.token = signed.Key
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
		if !data.user.Permissions.Admin {
			return http.StatusForbidden, nil
		}
		// Call the actual handler function with the updated context
		return fn(w, r, data)
	})
}

func wrapHandler(fn handleFunc) http.HandlerFunc {
	return wrapHandlerOpts(fn, wrapperOpts{})
}

type wrapperOpts struct {
	requestBasicAuth bool
}

func wrapHandlerOpts(fn handleFunc, opts wrapperOpts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &requestContext{}

		// Call the actual handler function and get status code and error
		status, err := fn(w, r, data)
		// Handle the error case if there is one
		if err != nil {
			// Create an error response in JSON format
			response := &HttpResponse{
				Status:  status, // Use the status code from the middleware
				Message: err.Error(),
			}

			if status == http.StatusUnauthorized && opts.requestBasicAuth {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
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

func withPermShareHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		if !d.user.Permissions.Share {
			return http.StatusForbidden, nil
		}
		return fn(w, r, d)
	})
}

func withPermShare(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withPermShareHelper(fn))
}

// Example of wrapping specific middleware functions for use with http.HandleFunc
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

func userWithoutOTP(fn handleFunc) http.HandlerFunc {
	return wrapHandler(userWithoutOTPhelper(fn))
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
		data.ctx = ctx
		// Call the handler and check for timeout
		status, err := fn(w, r, data)

		// Check if the context was cancelled due to timeout
		if ctx.Err() == context.DeadlineExceeded {
			return http.StatusRequestTimeout, fmt.Errorf("request timed out after %.0f seconds", timeout.Seconds())
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

// ResponseWriterWrapper wraps the standard http.ResponseWriter to capture the status code
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode  int
	wroteHeader bool
	PayloadSize int
	User        string
}

// WriteHeader captures the status code and ensures it's only written once
func (w *ResponseWriterWrapper) WriteHeader(statusCode int) {
	if !w.wroteHeader { // Prevent WriteHeader from being called multiple times
		if statusCode == 0 {
			statusCode = http.StatusInternalServerError
		}
		w.StatusCode = statusCode
		w.ResponseWriter.WriteHeader(statusCode)
		w.wroteHeader = true
	}
}

// Write is the method to write the response body and ensure WriteHeader is called
func (w *ResponseWriterWrapper) Write(b []byte) (int, error) {
	if !w.wroteHeader { // Default to 200 if WriteHeader wasn't called explicitly
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(b)
}

// Helper function to set the user in the ResponseWriterWrapper
func setUserInResponseWriter(w http.ResponseWriter, user *users.User) {
	// Wrap the response writer to set the user field
	if wrappedWriter, ok := w.(*ResponseWriterWrapper); ok {
		if user != nil {
			wrappedWriter.User = user.Username
		}
	}
}

// LoggingMiddleware logs each request and its status code.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// DEFER RECOVERY FUNCTION
		defer func() {
			if rcv := recover(); rcv != nil {
				// Log detailed information about the panic
				// Extract as much context as possible for logging
				method := r.Method
				url := r.URL.String()
				remoteAddr := r.RemoteAddr
				username := "unknown" // Default username

				// Attempt to get username from ResponseWriterWrapper if it's set
				if ww, ok := w.(*ResponseWriterWrapper); ok && ww.User != "" {
					username = ww.User
				}
				// Or try to get it from request context if your other middleware populates it
				// This depends on your context setup; example:
				// if dataCtx, ok := r.Context().Value("requestData").(*requestContext); ok && dataCtx.user != nil {
				// 	username = dataCtx.user.Username
				// }

				// Get Go-level stack trace
				buf := make([]byte, 16384)     // Increased buffer size for potentially long CGo traces
				n := runtime.Stack(buf, false) // false for current goroutine only
				stackTrace := string(buf[:n])

				logger.Errorf("PANIC RECOVERED: %v\nUser: %s\nMethod: %s\nURL: %s\nRemoteAddr: %s\nGo Stack Trace:\n%s",
					rcv, username, method, url, remoteAddr, stackTrace)

				// Attempt to send a 500 error response to the client
				// This is a best-effort; the connection might be broken or process too unstable.
				if ww, ok := w.(*ResponseWriterWrapper); ok { // Check if it's our wrapper
					if !ww.wroteHeader { // Only write if headers haven't been sent
						ww.Header().Set("Content-Type", "application/json; charset=utf-8")
						ww.WriteHeader(http.StatusInternalServerError)
					}
				} else {
					_, _ = renderJSON(w, r, &HttpResponse{
						Status:  500,
						Message: "A critical internal error occurred. Please try again later.",
					})
				}

				// IMPORTANT: After a SIGSEGV from C code, the process might be unstable.
				// Even if Go recovers, continuing to run the process is risky.
				// Consider a strategy to gracefully shut down or signal an external supervisor
				// to restart the process after logging. For now, this will allow other requests
				// to proceed if the process doesn't die, but be wary.
			}
		}() // End of deferred recovery function

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

		// Use the StatusCode from wrappedWriter, which might have been set to 500 by the recover logic
		logger.Api(wrappedWriter.StatusCode,
			fmt.Sprintf("%-7s | %3d | %-15s | %-12s | %-12s | \"%s\"",
				r.Method,
				wrappedWriter.StatusCode,
				r.RemoteAddr,
				truncUser,
				fmt.Sprintf("%vms", duration.Milliseconds()),
				fullURL))
	})
}

func renderJSON(w http.ResponseWriter, r *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	// Calculate size in KB
	payloadSizeKB := len(marsh) / 1024
	// Check if the client accepts gzip encoding and hasn't explicitly disabled it
	if acceptsGzip(r) && payloadSizeKB > 10 {
		// Enable gzip compression
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Encoding", "gzip")
		gz := gzip.NewWriter(w)
		defer gz.Close()

		if _, err := gz.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	} else {
		// Normal response without compression
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		if _, err := w.Write(marsh); err != nil {
			return http.StatusInternalServerError, err
		}
	}

	return 0, nil
}

func acceptsGzip(r *http.Request) bool {
	ae := r.Header.Get("Accept-Encoding")
	return ae != "" && strings.Contains(ae, "gzip")
}

func (w *ResponseWriterWrapper) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func getScheme(r *http.Request) string {
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}
