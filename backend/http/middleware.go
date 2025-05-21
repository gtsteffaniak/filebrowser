package http

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	jwt "github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/files"
	"github.com/gtsteffaniak/filebrowser/backend/auth"
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
			logger.Error("error fetching file info for share. hash=%v path=%v error=%v", hash, data.path, err)
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

// Middleware to retrieve and authenticate user
func withUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		if config.Auth.Methods.NoAuth {
			var err error
			// Retrieve the user from the store and store it in the context
			data.user, err = store.Users.Get(uint(1))
			if err != nil {
				logger.Error("no auth: %v", err)
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
				logger.Error("Error marshalling error response: %v", marshalErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Write the JSON error response
			if _, writeErr := w.Write(errorBytes); writeErr != nil {
				logger.Debug("Error writing error response: %v", writeErr)
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
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code.
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler.
		next.ServeHTTP(wrappedWriter, r)

		// Capture the full URL path including the query parameters.
		fullURL := r.URL.Path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}
		truncUser := wrappedWriter.User
		if len(truncUser) > 12 {
			truncUser = truncUser[:10] + ".."
		}
		duration := time.Since(start)
		logger.Api(
			fmt.Sprintf("%-7s | %3d | %-15s | %-12s | %-12s | \"%s\"",
				r.Method,
				wrappedWriter.StatusCode, // Captured status code
				r.RemoteAddr,
				truncUser,
				fmt.Sprintf("%vms", duration.Milliseconds()),
				fullURL), wrappedWriter.StatusCode)
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
