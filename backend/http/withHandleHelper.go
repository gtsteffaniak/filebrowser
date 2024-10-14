package http

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v4"
	"github.com/golang-jwt/jwt/v4/request"
	"github.com/gtsteffaniak/filebrowser/files"
	"github.com/gtsteffaniak/filebrowser/runner"
	"github.com/gtsteffaniak/filebrowser/users"
	"github.com/tomasen/realip"
)

// Middleware to handle file requests by hash and pass it to the handler
func withHashFile(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		d := &data{
			Runner:   &runner.Runner{Enabled: server.EnableExec, Settings: store.Settings},
			store:    store,
			settings: store.Settings,
			server:   server,
		}

		id, path := ifPathWithName(r)
		link, err := d.store.Share.GetByHash(id)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		if link.Hash != "" {
			status, err := authenticateShareRequest(r, link)
			if err != nil || status != 0 {
				http.Error(w, http.StatusText(status), status)
				return
			}
		}

		d.user = &users.PublicUser
		realPath, isDir, err := files.GetRealPath(d.user.Scope, link.Path, path)
		if err != nil {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		}

		file, err := files.FileInfoFaster(files.FileOptions{
			Path:       realPath,
			IsDir:      isDir,
			Modify:     d.user.Perm.Modify,
			Expand:     true,
			ReadHeader: d.server.TypeDetectionByHeader,
			Checker:    d,
			Token:      link.Token,
		})
		if err != nil {
			http.Error(w, http.StatusText(errToStatus(err)), errToStatus(err))
			return
		}

		d.raw = file
		status, err := fn(w, r, d)
		if err != nil || status >= 400 {
			clientIP := realip.FromRequest(r)
			log.Printf("%s: %v %s %v", r.URL.Path, status, clientIP, err)
			http.Error(w, http.StatusText(status), status)
		}
	}
}

// Middleware to extract the user and pass it to the handler
func withUser(fn handleFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Retrieve settings from the storage
		settings, err := store.Settings.Get()
		if err != nil {
			log.Fatalf("ERROR: couldn't get settings: %v\n", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		d := &data{
			Runner:   &runner.Runner{Enabled: server.EnableExec, Settings: settings},
			store:    store,
			settings: settings,
			server:   server,
		}

		keyFunc := func(token *jwt.Token) (interface{}, error) {
			return d.settings.Auth.Key, nil
		}

		var tk authToken
		token, err := request.ParseFromRequest(r, &extractor{}, keyFunc, request.WithClaims(&tk))
		if err != nil || !token.Valid {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Check if token has expired or needs updating
		expired := !tk.VerifyExpiresAt(time.Now().Add(time.Hour), true)
		updated := tk.IssuedAt != nil && tk.IssuedAt.Unix() < d.store.Users.LastUpdate(tk.User.ID)

		if expired || updated {
			w.Header().Add("X-Renew-Token", "true")
		}

		// Retrieve the user from the store
		d.user, err = d.store.Users.Get(d.server.Root, tk.User.ID)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Call the next handler with the `data` context
		status, err := fn(w, r, d)
		if err != nil || status >= 400 {
			clientIP := realip.FromRequest(r)
			log.Printf("%s: %v %s %v", r.URL.Path, status, clientIP, err)

			if status != 0 {
				txt := http.StatusText(status)
				http.Error(w, strconv.Itoa(status)+" "+txt, status)
			}
		}
	}
}

// Middleware to ensure the user is an admin
func withAdmin(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		// Ensure the user has admin permissions
		if !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		// Proceed to the actual handler if user is admin
		return fn(w, r, d)
	}
}

// Middleware to ensure the user is either the requested user or an admin
func withSelfOrAdmin(fn handleFunc) handleFunc {
	return func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		// Extract the user ID from the request
		id, err := getUserID(r) // Assuming getUserID extracts the user ID from the request
		if err != nil {
			return http.StatusInternalServerError, err
		}

		// Check if the current user is either the user being accessed or an admin
		if d.user.ID != id && !d.user.Perm.Admin {
			return http.StatusForbidden, nil
		}

		// Store the user ID or other relevant data in `d.raw` if needed
		d.raw = id

		// Proceed to the actual handler
		return fn(w, r, d)
	}
}
