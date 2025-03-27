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

	"github.com/gtsteffaniak/filebrowser/backend/common/logger"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/common/version"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage"

	httpSwagger "github.com/swaggo/http-swagger" // http-swagger middleware
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
	store     *storage.Storage
	config    *settings.Settings
	fileCache FileCache
	imgSvc    ImgService
	assetFs   fs.FS
)

func StartHttp(ctx context.Context, Service ImgService, storage *storage.Storage, cache FileCache, shutdownComplete chan struct{}) {

	store = storage
	fileCache = cache
	imgSvc = Service
	config = &settings.Config

	var err error

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
	api.HandleFunc("POST /auth/login", loginHandler)
	api.HandleFunc("GET /auth/signup", signupHandler)
	api.HandleFunc("POST /auth/renew", withUser(renewHandler))
	api.HandleFunc("PUT /auth/token", withUser(createApiKeyHandler))
	api.HandleFunc("GET /auth/token", withUser(createApiKeyHandler))
	api.HandleFunc("DELETE /auth/token", withUser(deleteApiKeyHandler))
	api.HandleFunc("GET /auth/tokens", withUser(listApiKeysHandler))

	// Resources routes
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(resourcePatchHandler))
	api.HandleFunc("GET /usage", withUser(diskUsage))
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
	api.HandleFunc("GET /public/publicUser", publicUserGetHandler)
	api.HandleFunc("GET /public/dl", withHashFile(publicRawHandler))
	api.HandleFunc("GET /public/share", withHashFile(publicShareHandler))

	// Settings routes
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withAdmin(settingsPutHandler))

	// Events routes
	api.HandleFunc("GET /events", withUser(func(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
		d.ctx = ctx // Pass the parent context to ensure proper shutdown
		return sseHandler(w, r, d)
	}))

	// Job routes
	api.HandleFunc("GET /job/{action}/{target}", withUser(getJobHandler))

	api.HandleFunc("GET /onlyoffice/config", withUser(onlyofficeClientConfigGetHandler))
	api.HandleFunc("POST /onlyoffice/callback", withUser(onlyofficeCallbackHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))
	apiPath := config.Server.BaseURL + "api"
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))

	// Static and index file handlers
	router.HandleFunc(fmt.Sprintf("GET %vstatic/", config.Server.BaseURL), staticFilesHandler)
	router.HandleFunc(config.Server.BaseURL, indexHandler)

	// health
	router.HandleFunc(fmt.Sprintf("GET %vhealth", config.Server.BaseURL), healthHandler)

	// Swagger
	router.Handle(fmt.Sprintf("%vswagger/", config.Server.BaseURL),
		httpSwagger.Handler(
			httpSwagger.URL(config.Server.BaseURL+"swagger/doc.json"), //The url pointing to API definition
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("none"),
			httpSwagger.DomID("swagger-ui"),
		),
	)

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
				logger.Fatal(fmt.Sprintf("Could not load certificate: %v", err))
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
			logger.Info(fmt.Sprintf("Running at               : %s", fullURL))

			// Create a TLS listener and serve
			listener, err := tls.Listen("tcp", srv.Addr, tlsConfig)
			if err != nil {
				logger.Fatal(fmt.Sprintf("Could not start TLS server: %v", err))
			}
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Fatal(fmt.Sprintf("Server error: %v", err))
			}
		} else {
			// Set HTTP scheme and the default port for HTTP
			scheme = "http"
			if config.Server.Port != 80 {
				port = fmt.Sprintf(":%d", config.Server.Port)
			}

			// Build the full URL with host and port
			fullURL := fmt.Sprintf("%s://localhost%s%s", scheme, port, config.Server.BaseURL)
			logger.Info(fmt.Sprintf("Running at               : %s", fullURL))

			// Start HTTP server
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				logger.Fatal(fmt.Sprintf("Server error: %v", err))
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
		logger.Error(fmt.Sprintf("HTTP server forced to shut down: %v", err))
	} else {
		logger.Info("HTTP server shut down gracefully.")
	}

	// Signal that shutdown is complete
	close(shutdownComplete)
}
