package http

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/go-logger/logger"
	// http-swagger middleware
)

// Embed the files in the frontend/dist directory
//
//go:embed embed/*
var assets embed.FS

// Custom dirFS to handle both embedded and non-embedded file systems
type dirFS struct {
	http.Dir
}

// Implement the Open method for dirFS, which wraps http.Dir
func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

var (
	store   *bolt.BoltStore
	config  *settings.Settings
	assetFs fs.FS
)

func StartHttp(ctx context.Context, storage *bolt.BoltStore, shutdownComplete chan struct{}) {
	store = storage
	config = &settings.Config

	// Check if http/dist directory exists to determine whether to use filesystem or embedded assets
	_, err := os.Stat("http/dist")
	embeddedFS := os.IsNotExist(err)

	// Dev mode enables development features like template hot-reloading
	devMode := os.Getenv("FILEBROWSER_DEVMODE") == "true"

	// --- START: ADD THIS DECRYPTION LOGIC ---
	if embeddedFS {
		// Embedded mode: Serve files from the embedded assets
		assetFs, err = fs.Sub(assets, "embed")
		if err != nil {
			logger.Fatal("Could not embed frontend. Does dist exist?")
		}
	} else {
		assetFs = dirFS{Dir: http.Dir("http/dist")}
	}

	// In development mode, we want to reload the templates on each request.
	// In production (embedded), we parse them once.
	templates := template.New("").Funcs(template.FuncMap{
		"marshal": func(v interface{}) (string, error) {
			a, err := json.Marshal(v)
			return string(a), err
		},
	})
	if !devMode {
		templates = template.Must(templates.ParseFS(assetFs, "public/index.html"))
	}
	templateRenderer = &TemplateRenderer{
		templates: templates,
		devMode:   devMode,
	}

	router := http.NewServeMux()
	// API group routing
	api := http.NewServeMux()
	// Public group routing (new structure)
	publicRoutes := http.NewServeMux()

	// User routes
	api.HandleFunc("GET /users", withUser(userGetHandler))
	api.HandleFunc("POST /users", withSelfOrAdmin(usersPostHandler))
	api.HandleFunc("PUT /users", withUser(userPutHandler))
	api.HandleFunc("DELETE /users", withSelfOrAdmin(userDeleteHandler))

	// Auth routes
	api.HandleFunc("POST /auth/login", userWithoutOTP(loginHandler))
	api.HandleFunc("POST /auth/logout", withUser(logoutHandler))
	api.HandleFunc("POST /auth/signup", withoutUser(signupHandler))
	api.HandleFunc("POST /auth/otp/generate", userWithoutOTP(generateOTPHandler))
	api.HandleFunc("POST /auth/otp/verify", userWithoutOTP(verifyOTPHandler))
	api.HandleFunc("POST /auth/renew", withUser(renewHandler))
	api.HandleFunc("PUT /auth/token", withUser(createApiKeyHandler))
	api.HandleFunc("GET /auth/token", withUser(createApiKeyHandler))
	api.HandleFunc("DELETE /auth/token", withUser(deleteApiKeyHandler))
	api.HandleFunc("GET /auth/tokens", withUser(listApiKeysHandler))
	api.HandleFunc("GET /auth/oidc/callback", wrapHandler(oidcCallbackHandler))
	api.HandleFunc("GET /auth/oidc/login", wrapHandler(oidcLoginHandler))

	// Resources routes
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(resourcePatchHandler))
	api.HandleFunc("GET /raw", withUser(rawHandler))
	api.HandleFunc("GET /preview", withUser(previewHandler))
	if version.Version == "testing" || version.Version == "untracked" {
		api.HandleFunc("GET /inspectIndex", inspectIndex)
		api.HandleFunc("GET /mockData", mockData)
	}
	// Access routes
	api.HandleFunc("GET /access", withAdmin(accessGetHandler))
	api.HandleFunc("POST /access", withAdmin(accessPostHandler))
	api.HandleFunc("DELETE /access", withAdmin(accessDeleteHandler))
	api.HandleFunc("GET /access/groups", withAdmin(groupGetHandler))
	api.HandleFunc("POST /access/group", withAdmin(groupPostHandler))
	api.HandleFunc("DELETE /access/group", withAdmin(groupDeleteHandler))
	// Create API sub-router for public API endpoints
	publicAPI := http.NewServeMux()
	// NEW PUBLIC ROUTES - All publicly accessible endpoints
	// Share management routes (require permission but are publicly accessible)
	publicRoutes.HandleFunc("GET /shares", withPermShare(shareListHandler))
	publicRoutes.HandleFunc("GET /share/direct", withPermShare(shareDirectDownloadHandler))
	publicRoutes.HandleFunc("GET /share", withPermShare(shareGetHandler))
	publicRoutes.HandleFunc("POST /share", withPermShare(sharePostHandler))
	publicRoutes.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))
	// Public API routes (hash-based authentication)
	publicAPI.HandleFunc("GET /raw", withHashFile(publicRawHandler))
	publicAPI.HandleFunc("GET /preview", withHashFile(publicPreviewHandler))
	publicAPI.HandleFunc("GET /resources", withHashFile(publicShareHandler))
	publicAPI.HandleFunc("GET /users", withUser(userGetHandler))
	// Settings routes
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))

	// Events routes
	api.HandleFunc("GET /events", withUser(func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		d.ctx = ctx // Pass the parent context to ensure proper shutdown
		return sseHandler(w, r, d)
	}))
	// Job routes
	api.HandleFunc("GET /jobs/{action}/{target}", withUser(getJobsHandler))

	api.HandleFunc("GET /onlyoffice/config", withUser(onlyofficeClientConfigGetHandler))
	publicAPI.HandleFunc("GET /onlyoffice/config", withHashFile(onlyofficeClientConfigGetHandler))
	api.HandleFunc("POST /onlyoffice/callback", withUser(onlyofficeCallbackHandler))
	publicAPI.HandleFunc("POST /onlyoffice/callback", withHashFile(onlyofficeCallbackHandler))
	api.HandleFunc("GET /onlyoffice/getToken", withUser(onlyofficeGetTokenHandler))
	publicAPI.HandleFunc("GET /onlyoffice/getToken", withHashFile(onlyofficeGetTokenHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))

	// Share routes (DEPRECATED - maintain for backwards compatibility)
	// These will redirect to the new /public/shares endpoints
	api.HandleFunc("GET /shares", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/shares"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	})
	api.HandleFunc("GET /share", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/share"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	})
	api.HandleFunc("POST /share", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, config.Server.BaseURL+"public/share", http.StatusPermanentRedirect)
	})
	api.HandleFunc("DELETE /share", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/share"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusPermanentRedirect)
	})

	api.HandleFunc("GET /public/raw", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/api/raw"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	})
	api.HandleFunc("GET /public/share", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/api/shared"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	})
	api.HandleFunc("GET /public/preview", func(w http.ResponseWriter, r *http.Request) {
		newURL := config.Server.BaseURL + "public/api/preview"
		if r.URL.RawQuery != "" {
			newURL += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, newURL, http.StatusMovedPermanently)
	})

	// Mount the public API sub-router
	publicRoutes.Handle("/api/", http.StripPrefix("/api", publicAPI))

	// Mount the route groups
	apiPath := config.Server.BaseURL + "api"
	publicPath := config.Server.BaseURL + "public"
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))
	router.Handle(publicPath+"/", http.StripPrefix(publicPath, publicRoutes))

	// Frontend share route redirect (DEPRECATED - maintain for backwards compatibility)
	router.HandleFunc(fmt.Sprintf("GET %vshare/", config.Server.BaseURL), withOrWithoutUser(redirectToShare))

	// New frontend share route handler - handle share page and any subpaths
	publicRoutes.HandleFunc("GET /share/", withOrWithoutUser(indexHandler))

	// Static and index file handlers
	staticPrefix := config.Server.BaseURL + "static/"
	router.Handle(staticPrefix, http.StripPrefix(staticPrefix, http.HandlerFunc(staticFilesHandler)))
	publicRoutes.Handle("GET /static/", http.StripPrefix("/static/", http.HandlerFunc(staticFilesHandler)))

	router.HandleFunc(config.Server.BaseURL, withOrWithoutUser(indexHandler))
	router.HandleFunc(fmt.Sprintf("GET %vhealth", config.Server.BaseURL), healthHandler)
	router.Handle(fmt.Sprintf("%vswagger/", config.Server.BaseURL), withUser(swaggerHandler))

	// redirect to baseUrl if not root
	if config.Server.BaseURL != "/" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, config.Server.BaseURL, http.StatusMovedPermanently)
		})
	}

	var scheme string
	port := ""
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%v", config.Server.Port),
		Handler: muxWithMiddleware(router),
	}
	go func() {
		// Determine whether to use HTTPS (TLS) or HTTP
		if config.Server.TLSCert != "" && config.Server.TLSKey != "" {
			// Load the TLS certificate and key
			cer, err := tls.LoadX509KeyPair(config.Server.TLSCert, config.Server.TLSKey)
			if err != nil {
				logger.Fatalf("Could not load certificate: %v", err)
			}

			// Create a custom TLS configuration
			tlsConfig := &tls.Config{
				MinVersion:   tls.VersionTLS12,
				Certificates: []tls.Certificate{cer},
			}

			// Set HTTPS scheme and default port for TLS
			scheme = "https"
			if config.Server.Port != 443 {
				port = fmt.Sprintf(":%d", config.Server.Port)
			}

			// Build the full URL with host and port
			fullURL := fmt.Sprintf("%s://localhost%s%s", scheme, port, config.Server.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			// Create a TLS listener and serve
			listener, err := tls.Listen("tcp", srv.Addr, tlsConfig)
			if err != nil {
				logger.Fatalf("Could not start TLS server: %v", err)
			}
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("Server error: %v", err)
			}
		} else {
			// Set HTTP scheme and the default port for HTTP
			scheme = "http"
			if config.Server.Port != 80 {
				port = fmt.Sprintf(":%d", config.Server.Port)
			}

			// Build the full URL with host and port
			fullURL := fmt.Sprintf("%s://localhost%s%s", scheme, port, config.Server.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			// Start HTTP server
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("Server error: %v", err)
			}
		}
	}()

	// Wait for context cancellation to shut down the server
	<-ctx.Done()
	logger.Info("Shutting down HTTP server...")

	// Persist in-memory state before shutting down the HTTP server
	if store != nil {
		if store.Share != nil {
			if err := store.Share.Flush(); err != nil {
				logger.Errorf("Failed to flush share storage: %v", err)
			}
		}
		if store.Access != nil {
			if err := store.Access.Flush(); err != nil {
				logger.Errorf("Failed to flush access storage: %v", err)
			}
		}
	}

	// Graceful shutdown with a timeout - 30 seconds, in case downloads are happening
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("HTTP server forced to shut down: %v", err)
	} else {
		logger.Info("HTTP server shut down gracefully.")
	}

	// Signal that shutdown is complete
	close(shutdownComplete)
}
