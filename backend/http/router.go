package http

import (
	"crypto/tls"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/version"

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

func StartHttp(Service ImgService, storage *storage.Storage, cache FileCache) {

	store = storage
	fileCache = cache
	imgSvc = Service
	config = &settings.Config

	var err error

	if embeddedFS {
		// Embedded mode: Serve files from the embedded assets
		assetFs, err = fs.Sub(assets, "embed")
		if err != nil {
			log.Fatal("Could not embed frontend. Does dist exist?")
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
	}

	// Share routes
	api.HandleFunc("GET /shares", withPermShare(shareListHandler))
	api.HandleFunc("GET /share", withPermShare(shareGetsHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))

	// Public routes
	api.HandleFunc("GET /public/publicUser", publicUserGetHandler)
	api.HandleFunc("GET /public/dl", withHashFile(publicDlHandler))
	api.HandleFunc("GET /public/share", withHashFile(publicShareHandler))

	// Settings routes
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withAdmin(settingsPutHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))
	apiPath := config.Server.BaseURL + "api"
	router.Handle(apiPath+"/", http.StripPrefix(apiPath, api))

	// Static and index file handlers
	router.HandleFunc(fmt.Sprintf("GET %vstatic/", config.Server.BaseURL), staticFilesHandler)
	router.HandleFunc(config.Server.BaseURL, indexHandler)

	// health
	router.HandleFunc(fmt.Sprintf("GET %vhealth/", config.Server.BaseURL), healthHandler)

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

	// Determine whether to use HTTPS (TLS) or HTTP
	if config.Server.TLSCert != "" && config.Server.TLSKey != "" {
		// Load the TLS certificate and key
		cer, err := tls.LoadX509KeyPair(config.Server.TLSCert, config.Server.TLSKey)
		if err != nil {
			log.Fatalf("could not load certificate: %v", err)
		}

		// Create a custom TLS listener
		tlsConfig := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cer},
		}

		// Set HTTPS scheme and default port for TLS
		scheme = "https"

		// Listen on TCP and wrap with TLS
		listener, err := tls.Listen("tcp", fmt.Sprintf(":%v", config.Server.Port), tlsConfig)
		if err != nil {
			log.Fatalf("could not start TLS server: %v", err)
		}
		if config.Server.Port != 443 {
			port = fmt.Sprintf(":%d", config.Server.Port)
		}
		// Build the full URL with host and port
		fullURL := fmt.Sprintf("%s://localhost%s%s", scheme, port, config.Server.BaseURL)
		log.Printf("Running at               : %s", fullURL)
		err = http.Serve(listener, muxWithMiddleware(router))
		if err != nil {
			log.Fatalf("could not start server: %v", err)
		}
	} else {
		// Set HTTP scheme and the default port for HTTP
		scheme = "http"
		if config.Server.Port != 80 {
			port = fmt.Sprintf(":%d", config.Server.Port)
		}
		// Build the full URL with host and port
		fullURL := fmt.Sprintf("%s://localhost%s%s", scheme, port, config.Server.BaseURL)
		log.Printf("Running at               : %s", fullURL)
		err := http.ListenAndServe(fmt.Sprintf(":%v", config.Server.Port), muxWithMiddleware(router))
		if err != nil {
			log.Fatalf("could not start server: %v", err)
		}
	}
}
