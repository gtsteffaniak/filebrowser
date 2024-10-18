package http

import (
	"net/http"
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
		// Extract the file ID and path from the request
		id, path := ifPathWithName(r)

		// Get the file link by hash
		link, err := store.Share.GetByHash(id)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return http.StatusNotFound, err
		}

		// Authenticate the share request if needed
		if link.Hash != "" {
			status, err := authenticateShareRequest(r, link)
			if err != nil || status != http.StatusOK {
				http.Error(w, http.StatusText(status), status)
				return status, err
			}
		}

		// Retrieve the user (using the public user by default)
		user := &users.PublicUser
		realPath, isDir, err := files.GetRealPath(user.Scope, link.Path, path)
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

// General middleware wrapper function
func wrapHandler(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := &requestContext{
			Runner: &runner.Runner{Enabled: config.Server.EnableExec, Settings: config},
		}

		// Set default status to 200
		status := http.StatusOK
		var err error

		// Call the actual handler function
		if status, err = fn(w, r, data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// If the status is not 200, respond with that status
		if status != http.StatusOK {
			http.Error(w, http.StatusText(status), status)
			return
		}

		// No explicit error or non-200 status means successful handling
		w.WriteHeader(http.StatusOK) // Ensure to send a 200 OK response
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
