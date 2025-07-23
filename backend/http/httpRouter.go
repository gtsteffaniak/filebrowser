package http

import (
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"text/template"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"
	"github.com/gtsteffaniak/go-logger/logger"
	// http-swagger middleware
)

// Embed the files in the frontend/dist directory
//
//go:embed embed/*
var assets embed.FS

// Boolean flag to determine whether to use the embedded FS or not
var embeddedFS = os.Getenv("FILEBROWSER_NO_EMBEDED") != "true"

// Custom dirFS to handle both embedded and non-embedded file systems
type dirFS struct {
	http.Dir
}

// Implement the Open method for dirFS, which wraps http.Dir
func (d dirFS) Open(name string) (fs.File, error) {
	return d.Dir.Open(name)
}

var (
	store  *storage.Storage
	config *settings.Settings
	//fileCache diskcache.Interface
	assetFs fs.FS
)

func StartHttp(ctx context.Context, storage *storage.Storage, shutdownComplete chan struct{}) {
	store = storage
	config = &settings.Config
	var err error
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

	templateRenderer = &TemplateRenderer{
		templates: template.Must(template.ParseFS(assetFs, "public/index.html")),
	}

	router := http.NewServeMux()
	// API group routing
	api := http.NewServeMux()

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

	// Share routes
	api.HandleFunc("GET /shares", withPermShare(shareListHandler))
	api.HandleFunc("GET /share", withPermShare(shareGetHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))

	// Public routes
	api.HandleFunc("GET /public/publicUser", withoutUser(publicUserGetHandler))
	api.HandleFunc("GET /public/dl", withHashFile(publicRawHandler))
	api.HandleFunc("GET /public/share", withHashFile(publicShareHandler))
	api.HandleFunc("GET /public/preview", withHashFile(publicPreviewHandler))

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
	api.HandleFunc("POST /onlyoffice/callback", withUser(onlyofficeCallbackHandler))
	api.HandleFunc("GET /onlyoffice/getToken", withUser(onlyofficeGetTokenHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))
	apiPath := config.Server.BaseURL + "api"
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))

	// Static and index file handlers
	router.HandleFunc(fmt.Sprintf("GET %vstatic/", config.Server.BaseURL), staticFilesHandler)
	router.HandleFunc(config.Server.BaseURL, indexHandler)

	// health
	router.HandleFunc(fmt.Sprintf("GET %vhealth", config.Server.BaseURL), healthHandler)

	// Swagger
	router.Handle(fmt.Sprintf("%vswagger/", config.Server.BaseURL), withUser(swaggerHandler))

	//	if config.Server.BaseURL != "/" {
	//		router.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
	//			http.Redirect(w, r, config.Server.BaseURL, http.StatusMovedPermanently)
	//		})
	//	}

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
