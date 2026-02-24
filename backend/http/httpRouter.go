package http

import (
	"context"
	"crypto/tls"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"text/template"
	"time"

	_ "net/http/pprof"

	"github.com/coreos/go-systemd/v22/activation"
	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/database/storage/bolt"
	"github.com/gtsteffaniak/filebrowser/backend/events"
	"github.com/gtsteffaniak/go-logger/logger"
	// http-swagger middleware
)

// Embed the files in the frontend/dist directory
//
//go:embed embed/*
var assets embed.FS

// GetEmbeddedAssets returns the embedded assets filesystem
func GetEmbeddedAssets() embed.FS {
	return assets
}

var (
	store   *bolt.BoltStore
	config  *settings.Settings
	assetFs fs.FS
)

func StartHttp(ctx context.Context, storage *bolt.BoltStore, shutdownComplete chan struct{}) {
	store = storage
	config = &settings.Config
	var err error
	// Start pprof server in a separate goroutine
	if settings.Env.IsDevMode {
		go func() {
			if err = http.ListenAndServe("localhost:6060", nil); err != nil {
				logger.Fatalf("pprof server error: %v", err)
			}
		}()
	}

	// Get the asset filesystem from fileutils
	assetFs = fileutils.GetAssetFS()
	if assetFs == nil {
		logger.Fatal("Asset filesystem not initialized. Call fileutils.InitAssetFS first.")
	}

	// In development mode, we want to reload the templates on each request.
	// In production (embedded), we parse them once.
	templates := template.New("").Funcs(template.FuncMap{
		"marshal": func(v interface{}) (string, error) {
			a, err := json.Marshal(v)
			return string(a), err
		},
	})
	if !settings.Env.IsDevMode {
		templates = template.Must(templates.ParseFS(assetFs, "public/index.html"))
	}
	templateRenderer = &TemplateRenderer{
		templates: templates,
		devMode:   settings.Env.IsDevMode,
	}

	// Core routing
	router := http.NewServeMux()
	api := http.NewServeMux()
	publicRoutes := http.NewServeMux()
	publicApi := http.NewServeMux()

	// Feature-specific muxes
	userMux := http.NewServeMux()
	authMux := http.NewServeMux()
	resourcesMux := http.NewServeMux()
	accessMux := http.NewServeMux()
	shareMux := http.NewServeMux()
	settingsMux := http.NewServeMux()
	toolsMux := http.NewServeMux()
	officeMux := http.NewServeMux()
	sharePublicMux := http.NewServeMux()

	// ========================================
	// User Routes - /api/users/ or /public/api/users/
	// ========================================
	userMux.HandleFunc("GET /", withUser(userGetHandler))
	userMux.HandleFunc("POST /", withSelfOrAdmin(usersPostHandler))
	userMux.HandleFunc("PUT /", withUser(userPutHandler))
	userMux.HandleFunc("DELETE /", withSelfOrAdmin(userDeleteHandler))
	publicApi.HandleFunc("GET /users", withUser(userGetHandler))

	// ========================================
	// Auth Routes - /api/auth/ or /public/api/auth/
	// ========================================
	authMux.HandleFunc("POST /login", userWithoutOTP(loginHandler))
	authMux.HandleFunc("POST /logout", withOrWithoutUser(logoutHandler))
	authMux.HandleFunc("POST /signup", withoutUser(signupHandler))
	authMux.HandleFunc("POST /otp/generate", userWithoutOTP(generateOTPHandler))
	authMux.HandleFunc("POST /otp/verify", userWithoutOTP(verifyOTPHandler))
	authMux.HandleFunc("POST /renew", withUser(renewHandler))
	authMux.HandleFunc("PUT /token", withUser(createApiKeyHandler))
	authMux.HandleFunc("GET /token", withUser(createApiKeyHandler))
	authMux.HandleFunc("DELETE /token", withUser(deleteApiKeyHandler))
	authMux.HandleFunc("GET /tokens", withUser(listApiKeysHandler))
	authMux.HandleFunc("GET /oidc/callback", wrapHandler(oidcCallbackHandler))
	authMux.HandleFunc("GET /oidc/login", wrapHandler(oidcLoginHandler))

	// ========================================
	// Resources Routes - /api/resources/ or /public/api/resources
	// ========================================
	resourcesMux.HandleFunc("GET /", withUser(resourceGetHandler))
	resourcesMux.HandleFunc("GET /items", withUser(itemsGetHandler))
	resourcesMux.HandleFunc("DELETE /", withUser(resourceDeleteHandler))
	resourcesMux.HandleFunc("POST /", withUser(resourcePostHandler))
	resourcesMux.HandleFunc("PUT /", withUser(resourcePutHandler))
	resourcesMux.HandleFunc("PATCH /", withUser(resourcePatchHandler))
	resourcesMux.HandleFunc("DELETE /bulk", withUser(resourceBulkDeleteHandler))
	resourcesMux.HandleFunc("POST /archive", withUser(archiveCreateHandler))
	resourcesMux.HandleFunc("POST /unarchive", withUser(unarchiveHandler))
	resourcesMux.HandleFunc("GET /raw", withUser(rawHandler))
	resourcesMux.HandleFunc("GET /preview", withTimeout(60*time.Second, withUserHelper(previewHandler)))
	resourcesMux.HandleFunc("GET /media/subtitles", withUser(subtitlesHandler))
	publicApi.HandleFunc("GET /resources", withHashFile(publicGetResourceHandler))
	publicApi.HandleFunc("GET /resources/items", withHashFile(publicItemsGetHandler))
	publicApi.HandleFunc("POST /resources", withHashFile(publicUploadHandler))
	publicApi.HandleFunc("PUT /resources", withHashFile(publicPutHandler))
	publicApi.HandleFunc("DELETE /resources", withHashFile(publicDeleteHandler))
	publicApi.HandleFunc("DELETE /resources/bulk", withHashFile(publicBulkDeleteHandler))
	publicApi.HandleFunc("PATCH /resources", withHashFile(publicPatchHandler))

	// Legacy routes (backwards compatibility for downloads)
	api.HandleFunc("GET /raw", withUser(rawHandler))
	publicApi.HandleFunc("GET /raw", withHashFile(publicRawHandler))

	// ========================================
	// Access Routes - /api/access/
	// ========================================
	accessMux.HandleFunc("GET /", withAdmin(accessGetHandler))
	accessMux.HandleFunc("POST /", withAdmin(accessPostHandler))
	accessMux.HandleFunc("PATCH /", withAdmin(accessPatchHandler))
	accessMux.HandleFunc("DELETE /", withAdmin(accessDeleteHandler))
	accessMux.HandleFunc("GET /groups", withAdmin(groupGetHandler))
	accessMux.HandleFunc("POST /group", withAdmin(groupPostHandler))
	accessMux.HandleFunc("DELETE /group", withAdmin(groupDeleteHandler))

	// ========================================
	// Share Routes - /api/share/ (no public routes)
	// ========================================
	shareMux.HandleFunc("GET /", withPermShare(shareListHandler))
	shareMux.HandleFunc("GET /direct", withPermShare(shareDirectDownloadHandler))
	shareMux.HandleFunc("GET /info", withPermShare(shareGetHandler))
	shareMux.HandleFunc("POST /", withPermShare(sharePostHandler))
	shareMux.HandleFunc("PATCH /", withPermShare(sharePatchHandler))
	shareMux.HandleFunc("DELETE /", withPermShare(shareDeleteHandler))

	// ========================================
	// Settings Routes - /api/settings/ (no public routes)
	// ========================================
	settingsMux.HandleFunc("GET /", withAdmin(settingsGetHandler))
	settingsMux.HandleFunc("GET /config", withAdmin(settingsConfigHandler))
	settingsMux.HandleFunc("GET /sources", withAdmin(getSourceInfoHandler))

	// ========================================
	// Tools Routes - /api/tools/ (no public routes)
	// ========================================
	toolsMux.HandleFunc("GET /search", withUser(searchHandler))
	toolsMux.HandleFunc("GET /duplicates", withUser(duplicatesHandler))
	toolsMux.HandleFunc("GET /watch", withUser(fileWatchHandler))
	toolsMux.HandleFunc("GET /watch/sse", withUser(fileWatchSSEHandler))

	// ========================================
	// OnlyOffice Routes - /api/office/ or /public/api/onlyoffice/
	// ========================================
	officeMux.HandleFunc("GET /config", withUser(onlyofficeClientConfigGetHandler))
	officeMux.HandleFunc("POST /callback", withUser(onlyofficeCallbackHandler))
	officeMux.HandleFunc("GET /callback", withUser(onlyofficeCallbackHandler))
	publicApi.HandleFunc("POST /onlyoffice/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /onlyoffice/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /onlyoffice/config", withHashFile(onlyofficeClientConfigGetHandler))

	// ========================================
	// Share Public Routes - /public/api/share/
	// ========================================
	sharePublicMux.HandleFunc("GET /info", withOrWithoutUser(shareInfoHandler))
	sharePublicMux.HandleFunc("GET /image", withHashFile(getShareImage))

	// ========================================
	// Misc Routes
	// ========================================
	// General health check
	api.HandleFunc("GET /health", healthHandler)
	// Event streaming
	api.HandleFunc("GET /events", withUser(sseHandler))
	// Dev-only routes
	if settings.Env.IsDevMode {
		api.HandleFunc("GET /inspectIndex", inspectIndex)
		api.HandleFunc("GET /mockData", mockData)
	}

	// ========================================
	// Mount Sub-Muxes
	// ========================================
	api.Handle("/users/", http.StripPrefix("/users", userMux))
	api.Handle("/auth/", http.StripPrefix("/auth", authMux))
	api.Handle("/resources/", http.StripPrefix("/resources", resourcesMux))
	api.Handle("/access/", http.StripPrefix("/access", accessMux))
	api.Handle("/share/", http.StripPrefix("/share", shareMux))
	api.Handle("/shares/", http.StripPrefix("/shares", shareMux))
	api.Handle("/settings/", http.StripPrefix("/settings", settingsMux))
	api.Handle("/tools/", http.StripPrefix("/tools", toolsMux))
	api.Handle("/office/", http.StripPrefix("/office", officeMux))

	// Mount public API
	publicRoutes.Handle("/api/", http.StripPrefix("/api", publicApi))

	// ========================================
	// Configure Main Router
	// ========================================
	apiPath := config.Server.BaseURL + "api"
	publicPath := config.Server.BaseURL + "public"
	webDavPath := config.Server.BaseURL + "dav"

	// Mount primary API and public routes
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))
	router.Handle(publicPath+"/", http.StripPrefix(publicPath, publicRoutes))

	// WebDAV handler
	if !config.Server.DisableWebDAV {
		// Uses Basic Auth where password is JWT token
		// Note: do not trim /dav prefix here - webdav library requires it
		router.Handle(webDavPath+"/{source}/{path...}", withBasicAuth(webDAVHandler))
	}

	// Frontend share route redirect (DEPRECATED - maintain for backwards compatibility)
	// TODO: Playwright tests need updating to remove this redirect
	router.HandleFunc(fmt.Sprintf("GET %vshare/", config.Server.BaseURL), withOrWithoutUser(redirectToShare))

	// New frontend share route handler
	publicRoutes.HandleFunc("GET /share/", withOrWithoutUser(indexHandler))

	// Static assets
	publicRoutes.Handle("GET /static/", http.HandlerFunc(staticAssetHandler))
	router.HandleFunc("GET /favicon.svg", http.HandlerFunc(staticAssetHandler))

	// Index and utility routes
	router.HandleFunc(config.Server.BaseURL, withOrWithoutUser(indexHandler))
	router.HandleFunc(fmt.Sprintf("GET %vhealth", config.Server.BaseURL), healthHandler)
	router.Handle(fmt.Sprintf("%vswagger/", config.Server.BaseURL), withUser(swaggerHandler))

	// Base URL redirect (non-root deployments)
	if config.Server.BaseURL != "/" {
		router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, config.Server.BaseURL, http.StatusMovedPermanently)
		})
	}

	var scheme string
	port := ""
	srv := &http.Server{
		Addr:    fmt.Sprintf("%v:%v", config.Server.ListenAddress, config.Server.Port),
		Handler: muxWithMiddleware(router),
	}
	listenAddress := config.Server.ListenAddress
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
			fullURL := fmt.Sprintf("%s://%s%s%s", scheme, listenAddress, port, config.Server.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			// Attempt to get listener from socket activation with TLS configuration
			var listener net.Listener

			socketActivationListeners, err := activation.TLSListeners(tlsConfig)
			if err == nil && len(socketActivationListeners) > 0 {
				listener = socketActivationListeners[0]
				logger.Debug("Socket activation detected. Listening address is being controlled by systemd.")
			} else if err != nil {
				// if Socket Activation fails we can just fall back to create our own sockets as normal.
				// so is only interesting to those who are debugging the app.
				logger.Debugf("Socket activation failed: %v", err)
			}

			// Create our own TLS listener if socket activation is not available.
			if listener == nil {
				listener, err = tls.Listen("tcp", srv.Addr, tlsConfig)
				if err != nil {
					logger.Fatalf("Could not start TLS server: %v", err)
				}
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
			fullURL := fmt.Sprintf("%s://%s%s%s", scheme, listenAddress, port, config.Server.BaseURL)
			logger.Infof("Running at               : %s", fullURL)

			var listener net.Listener

			// Attempt to get the listener from socket activation
			socketActivationListeners, err := activation.Listeners()
			if err == nil && len(socketActivationListeners) > 0 {
				listener = socketActivationListeners[0]
				logger.Debug("Socket activation detected. Listening address is being controlled by systemd.")
			} else if err != nil {
				// if Socket Activation fails we can just fall back to create our own sockets as normal.
				// so is only interesting to those who are debugging the app.
				logger.Debugf("Socket activation failed: %v", err)
			}

			// Create our own listener if socket activation is not available.
			if listener == nil {
				// Replicate the behaviour of ListenAndServe
				addr := srv.Addr
				if addr == "" {
					addr = ":http"
				}

				listener, err = net.Listen("tcp", addr)
				if err != nil {
					logger.Fatalf("Server error: %v", err)
				}
			}

			// Start HTTP server
			if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
				logger.Fatalf("Server error: %v", err)
			}
		}
	}()

	// Wait for context cancellation to shut down the server
	<-ctx.Done()
	logger.Info("Shutting down HTTP server...")

	// Close all SSE sessions
	events.Shutdown()

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
		if store.Indexing != nil {
			if err := store.Indexing.Flush(); err != nil {
				logger.Errorf("Failed to flush indexing storage: %v", err)
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
