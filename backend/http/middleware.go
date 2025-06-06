package http

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
	"github.com/gtsteffaniak/filebrowser/backend/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

type requestContext struct {
	user     *users.User
	raw      interface{}
	fileInfo iteminfo.ExtendedFileInfo
	path     string
	token    string
	share    *share.Link
	ctx      context.Context
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
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		path := r.URL.Query().Get("path")
		hash := r.URL.Query().Get("hash")
		// Retrieve the user (using the public user by default)
		data.user = &users.PublicUser
		// Get the file link by hash
		link, err := store.Share.GetByHash(hash)
		if err != nil {
			return http.StatusNotFound, fmt.Errorf("share not found")
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
		data.path = strings.TrimSuffix(link.Path, "/") + "/" + strings.TrimPrefix(path, "/")
		if path == "" || path == "/" {
			data.path = link.Path
		}

		source, ok := config.Server.SourceMap[link.Source]
		if !ok {
			return http.StatusNotFound, fmt.Errorf("source not found")
		}
		// Get file information with options
		file, err := FileInfoFasterFunc(iteminfo.FileOptions{
			Path:   data.path,
			Source: source.Name,
			Modify: false,
			Expand: true,
		})
		file.Token = link.Token
		if err != nil {
			logger.Errorf("error fetching file info for share. hash=%v path=%v error=%v", hash, data.path, err)
			return errToStatus(err), fmt.Errorf("error fetching share from server")
		}
		// Set the file info in the `data` object
		data.raw = file
		// Call the next handler with the data
		return fn(w, r, data)
	}
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
		proxyUser := r.Header.Get(config.Auth.Methods.ProxyAuth.Header)
		if config.Auth.Methods.ProxyAuth.Enabled && proxyUser != "" {
			user, err := setupProxyUser(r, &requestContext{}, proxyUser)
			if err != nil {
				return 401, errors.ErrUnauthorized
			}
			d.user = user
		} else if username == "" || password == "" {
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
			return fn(w, r, data)
		}
		proxyUser := r.Header.Get(config.Auth.Methods.ProxyAuth.Header)
		if config.Auth.Methods.ProxyAuth.Enabled && proxyUser != "" {
			user, err := setupProxyUser(r, data, proxyUser)
			if err != nil {
				return http.StatusForbidden, err
			}
			data.user = user
			setUserInResponseWriter(w, data.user)
			return fn(w, r, data)
		}
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return []byte(config.Auth.Key), nil
		}
		tokenString, err := extractToken(r)
		if err != nil {
			return http.StatusUnauthorized, err
		}
		data.token = tokenString
		var tk users.AuthToken
		token, err := jwt.ParseWithClaims(tokenString, &tk, keyFunc)
		if err != nil {
			return http.StatusUnauthorized, fmt.Errorf("error processing token, %v", err)
		}
		if !token.Valid {
			return http.StatusUnauthorized, fmt.Errorf("invalid token")
		}
		if auth.IsRevokedApiKey(tk.Key) || tk.Expires < time.Now().Unix() {
			return http.StatusUnauthorized, fmt.Errorf("token expired or revoked")
		}
		// Check if the token is about to expire and send a header to renew it
		if tk.Expires < time.Now().Add(time.Hour).Unix() {
			w.Header().Add("X-Renew-Token", "true")
		}

		// Retrieve the user from the store and store it in the context
		data.user, err = store.Users.Get(tk.BelongsTo)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		setUserInResponseWriter(w, data.user)

		// Call the handler function, passing in the context
		return fn(w, r, data)
	}
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

func withoutUser(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withoutUserHelper(fn))
}

func userWithoutOTP(fn handleFunc) http.HandlerFunc {
	return wrapHandler(userWithoutOTPhelper(fn))
}

func withSelfOrAdmin(fn handleFunc) http.HandlerFunc {
	return wrapHandler(withSelfOrAdminHelper(fn))
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
