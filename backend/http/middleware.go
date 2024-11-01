package http

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/runner"
	"github.com/gtsteffaniak/filebrowser/users"
)

type requestContext struct {
	user *users.User
	*runner.Runner
	raw interface{}
}

// Updated handleFunc to match the new signature
type handleFunc func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error)

// Middleware to handle file requests by hash and pass it to the handler
func withHashFileHelper(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, data *requestContext) (int, error) {
		adjustedRestPath := strings.TrimPrefix(r.URL.Path, "/public/share/")
		splitPath := strings.SplitN(adjustedRestPath, "/", 2)
		hash := splitPath[0]
		subPath := ""
		if len(splitPath) > 1 {
			subPath = splitPath[1]
		}
		// Get the file link by hash
		link, err := store.Share.GetByHash(hash)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return http.StatusNotFound, err
		}
		// Authenticate the share request if needed
		var status int
		if link.Hash != "" {
			status, err = authenticateShareRequest(r, link)
			if err != nil || status != http.StatusOK {
				http.Error(w, http.StatusText(status), status)
				return status, err
			}
		}
		// Retrieve the user (using the public user by default)
		user := &users.PublicUser
		realPath, isDir, err := files.GetRealPath(user.Scope, link.Path+"/"+subPath)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return http.StatusNotFound, err
		}

		// Get file information with options
		file, err := files.FileInfoFaster(files.FileOptions{
			Path:       realPath,
			IsDir:      isDir,
			Modify:     user.Perm.Modify,
			Expand:     true,
			ReadHeader: config.Server.TypeDetectionByHeader,
			Checker:    user, // Call your checker function here
			Token:      link.Token,
		})
		if err != nil {
			http.Error(w, http.StatusText(errToStatus(err)), errToStatus(err))
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
		var tk authToken
		token, err := request.ParseFromRequest(r, &extractor{}, keyFunc, request.WithClaims(&tk))
		if err != nil || !token.Valid {
			return http.StatusUnauthorized, nil
		}
		expired := !tk.VerifyExpiresAt(time.Now().Add(time.Hour), true)
		updated := tk.IssuedAt != nil && tk.IssuedAt.Unix() < store.Users.LastUpdate(tk.User.ID)

		if expired || updated {
			w.Header().Add("X-Renew-Token", "true")
		}
		// Retrieve the user from the store and store it in the context
		data.user, err = store.Users.Get(config.Server.Root, tk.User.ID)
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
		// Extract user ID from the request (e.g., from URL parameters)
		id, err := getUserID(r)
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// Check if the current user is the same as the requested user or if they are an admin
		if data.user.ID != id && !data.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		// If authorized, set the raw field with the requested ID
		data.raw = id

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
			// Log the actual error to the server logs
			log.Printf("Error: %v", err)

			// If the status is not explicitly set, default to 500
			if status == http.StatusOK {
				status = http.StatusInternalServerError
			}

			// Send the error response with the status and error message
			http.Error(w, err.Error(), status)
			return
		}

		// If the status is not 200, return the appropriate status
		if status != http.StatusOK {
			http.Error(w, http.StatusText(status), status)
			return
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
		if wrappedWriter.StatusCode >= 400 && wrappedWriter.StatusCode < 500 {
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
