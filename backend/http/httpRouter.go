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

	// ========================================
	// User Routes - /api/users/ (with public routes)
	// ========================================
	api.HandleFunc("GET /users", withUser(userGetHandler))
	api.HandleFunc("POST /users", withSelfOrAdmin(usersPostHandler))
	api.HandleFunc("PUT /users", withUser(userPutHandler))
	api.HandleFunc("DELETE /users", withSelfOrAdmin(userDeleteHandler))
	publicApi.HandleFunc("GET /users", withUser(userGetHandler))

	// ========================================
	// Auth Routes - /api/auth/
	// ========================================
	api.HandleFunc("POST /auth/login", userWithoutOTP(loginHandler))
	api.HandleFunc("POST /auth/logout", withOrWithoutUser(logoutHandler))
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

	// ========================================
	// Resources Routes - /api/resources/ (with public routes)
	// ========================================
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("GET /resources/items", withUser(itemsGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(resourcePatchHandler))
	api.HandleFunc("DELETE /resources/bulk", withUser(resourceBulkDeleteHandler))
	api.HandleFunc("POST /resources/archive", withUser(archiveCreateHandler))
	api.HandleFunc("POST /resources/unarchive", withUser(unarchiveHandler))
	api.HandleFunc("GET /resources/raw", withUser(rawHandler))
	api.HandleFunc("GET /resources/preview", withTimeout(60*time.Second, withUserHelper(previewHandler)))
	publicApi.HandleFunc("GET /resources", withHashFile(publicGetResourceHandler))
	publicApi.HandleFunc("GET /resources/items", withHashFile(publicItemsGetHandler))
	publicApi.HandleFunc("POST /resources", withHashFile(publicUploadHandler))
	publicApi.HandleFunc("PUT /resources", withHashFile(publicPutHandler))
	publicApi.HandleFunc("DELETE /resources", withHashFile(publicDeleteHandler))
	publicApi.HandleFunc("DELETE /resources/bulk", withHashFile(publicBulkDeleteHandler))
	publicApi.HandleFunc("PATCH /resources", withHashFile(publicPatchHandler))
	publicApi.HandleFunc("GET /resources/preview", withHashFile(publicPreviewHandler))
	// Legacy routes (backwards compatibility)
	api.HandleFunc("GET /raw", withUser(rawHandler))
	publicApi.HandleFunc("GET /raw", withHashFile(publicRawHandler))

	// ========================================
	// Access Routes - /api/access/
	// ========================================
	api.HandleFunc("GET /access", withAdmin(accessGetHandler))
	api.HandleFunc("POST /access", withAdmin(accessPostHandler))
	api.HandleFunc("PATCH /access", withAdmin(accessPatchHandler))
	api.HandleFunc("DELETE /access", withAdmin(accessDeleteHandler))
	api.HandleFunc("GET /access/groups", withAdmin(groupGetHandler))
	api.HandleFunc("POST /access/group", withAdmin(groupPostHandler))
	api.HandleFunc("DELETE /access/group", withAdmin(groupDeleteHandler))

	// ========================================
	// Share Routes - /api/share/
	// ========================================
	api.HandleFunc("GET /share/list", withPermShare(shareListHandler))
	api.HandleFunc("GET /share/direct", withPermShare(shareDirectDownloadHandler))
	api.HandleFunc("GET /share", withPermShare(shareGetHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("PATCH /share", withPermShare(sharePatchHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))
	publicApi.HandleFunc("GET /share/info", withOrWithoutUser(shareInfoHandler))
	publicApi.HandleFunc("GET /share/image", withHashFile(getShareImage))

	// ========================================
	// Settings Routes - /api/settings/
	// ========================================
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("GET /settings/config", withAdmin(settingsConfigHandler))
	api.HandleFunc("GET /settings/sources", withUser(getSourceInfoHandler))

	// ========================================
	// Tools Routes - /api/tools/
	// ========================================
	api.HandleFunc("GET /tools/search", withUser(searchHandler))
	api.HandleFunc("GET /tools/duplicateFinder", withUser(duplicatesHandler))
	api.HandleFunc("GET /tools/fileWatcher", withUser(fileWatchHandler))
	api.HandleFunc("GET /tools/fileWatcher/sse", withUser(fileWatchSSEHandler))

	// ========================================
	// Media Routes - /api/media/
	// ========================================
	api.HandleFunc("GET /media/subtitles", withUser(subtitlesHandler))

	// ========================================
	// OnlyOffice Routes - /api/office/ (with public routes)
	// ========================================
	api.HandleFunc("GET /office/config", withUser(onlyofficeClientConfigGetHandler))
	api.HandleFunc("POST /office/callback", withUser(onlyofficeCallbackHandler))
	api.HandleFunc("GET /office/callback", withUser(onlyofficeCallbackHandler))
	publicApi.HandleFunc("POST /office/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /office/callback", withHashFile(onlyofficeCallbackHandler))
	publicApi.HandleFunc("GET /office/config", withHashFile(onlyofficeClientConfigGetHandler))

	// ========================================
	// Misc Routes
	// ========================================
	api.HandleFunc("GET /health", healthHandler)
	api.HandleFunc("GET /events", withUser(sseHandler))
	if settings.Env.IsDevMode {
		api.HandleFunc("GET /inspectIndex", inspectIndex)
		api.HandleFunc("GET /mockData", mockData)
	}

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
