package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/runner"
	"github.com/gtsteffaniak/filebrowser/users"
)

type requestContext struct {
	user *users.User
	*runner.Runner
	raw interface{}
}

type HttpResponse struct {
	Status  int    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
	Token   string `json:"token,omitempty"`
}

// Updated handleFunc to match the new signature
type handleFunc func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error)

// Middleware to handle file requests by hash and pass it to the handler
func withHashFileHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		path := r.URL.Query().Get("path")
		hash := r.URL.Query().Get("hash")
		data.user = &users.PublicUser

		// Get the file link by hash
		link, err := store.Share.GetByHash(hash)
		if err != nil {
			return http.StatusNotFound, err
		}
		// Authenticate the share request if needed
		var status int
		if link.Hash != "" {
			status, err = authenticateShareRequest(r, link)
			if err != nil || status != http.StatusOK {
				return status, err
			}
		}
		// Retrieve the user (using the public user by default)
		user := &users.PublicUser

		// Get file information with options
		file, err := files.FileInfoFaster(files.FileOptions{
			Path:       filepath.Join(user.Scope, link.Path+"/"+path),
			Modify:     user.Perm.Modify,
			Expand:     true,
			ReadHeader: config.Server.TypeDetectionByHeader,
			Checker:    user, // Call your checker function here
			Token:      link.Token,
		})
		if err != nil {
			return errToStatus(err), err
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
		if !data.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		// Proceed to the actual handler if the user is admin
		return fn(w, r, data)
	})
}

// Middleware to retrieve and authenticate user
func withUserHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return config.Auth.Key, nil
		}
		tokenString, err := extractToken(r)
		if err != nil {
			return http.StatusUnauthorized, err
		}

		var tk users.AuthToken
		token, err := jwt.ParseWithClaims(tokenString, &tk, keyFunc)
		if err != nil {
			return http.StatusUnauthorized, fmt.Errorf("error processing token, %v", err)
		}
		if !token.Valid {
			return http.StatusUnauthorized, fmt.Errorf("invalid token")
		}
		if isRevokedApiKey(tk.Key) || tk.Expires < time.Now().Unix() {
			return http.StatusUnauthorized, fmt.Errorf("token expired or revoked")
		}
		// Check if the token is about to expire and send a header to renew it
		if tk.Expires < time.Now().Add(time.Hour).Unix() {
			w.Header().Add("X-Renew-Token", "true")
		}
		// Retrieve the user from the store and store it in the context
		data.user, err = store.Users.Get(config.Server.Root, tk.BelongsTo)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		// Call the handler function, passing in the context
		return fn(w, r, data)
	}
}

// Middleware to ensure the user is either the requested user or an admin
func withSelfOrAdminHelper(fn handleFunc) handleFunc {
	return withUserHelper(func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		// Check if the current user is the same as the requested user or if they are an admin
		if !data.user.Perm.Admin {
			return http.StatusForbidden, nil
		}
		// Call the actual handler function with the updated context
		return fn(w, r, data)
	})
}

func wrapHandler(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &requestContext{
			Runner: &runner.Runner{Enabled: config.Server.EnableExec, Settings: config},
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
				log.Printf("Error marshalling error response: %v", marshalErr)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			// Write the JSON error response
			if _, writeErr := w.Write(errorBytes); writeErr != nil {
				log.Printf("Error writing error response: %v", writeErr)
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
		if !d.user.Perm.Share {
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

// LoggingMiddleware logs each request and its status code
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrappedWriter, r)

		// Determine the color based on the status code
		color := "\033[32m" // Default green color
		if wrappedWriter.StatusCode >= 300 && wrappedWriter.StatusCode < 500 {
			color = "\033[33m" // Yellow for client errors (4xx)
		} else if wrappedWriter.StatusCode >= 500 {
			color = "\033[31m" // Red for server errors (5xx)
		}
		// Capture the full URL path including the query parameters
		fullURL := r.URL.Path
		if r.URL.RawQuery != "" {
			fullURL += "?" + r.URL.RawQuery
		}

		// Log the request and its status code
		log.Printf("%s%-7s | %3d | %-15s | %-12s | \"%s\"%s",
			color,
			r.Method,
			wrappedWriter.StatusCode, // Now capturing the correct status
			r.RemoteAddr,
			time.Since(start).String(),
			fullURL,
			"\033[0m", // Reset color
		)
	})
}

func renderJSON(w http.ResponseWriter, _ *http.Request, data interface{}) (int, error) {
	marsh, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	if _, err := w.Write(marsh); err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}
