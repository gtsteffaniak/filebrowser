package http

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"

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

type modifyRequest struct {
	What  string   `json:"what"`  // Answer to: what data type?
	Which []string `json:"which"` // Answer to: which fields?
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
	config.Server.Clean()

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
	api.HandleFunc("GET /users", withAdmin(userGetHandler))
	api.HandleFunc("POST /users", withAdmin(usersPostHandler))
	api.HandleFunc("PUT /users", withSelfOrAdmin(userPutHandler))
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
	api.HandleFunc("GET /resources/", withUser(resourceGetHandler))
	api.HandleFunc("DELETE /resources/", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources/", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources/", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resource/", withUser(resourcePatchHandler))
	api.HandleFunc("GET /usage/", withUser(diskUsage))
	api.HandleFunc("GET /raw", withUser(rawHandler))
	api.HandleFunc("GET /preview", withUser(previewHandler))

	// Share routes
	api.HandleFunc("GET /shares", withPermShare(shareListHandler))
	api.HandleFunc("GET /share/", withPermShare(shareGetsHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))

	// Public routes
	api.HandleFunc("GET /public/publicUser", publicUserGetHandler)
	api.HandleFunc("GET /public/dl", withHashFile(publicDlHandler))
	api.HandleFunc("GET /public/share/", withHashFile(publicShareHandler))

	// Settings routes
	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withAdmin(settingsPutHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))

	router.Handle("/api/", http.StripPrefix("/api", api))

	// Static and index file handlers
	router.HandleFunc("GET /static/", staticFilesHandler)
	router.HandleFunc("/", indexHandler)

	// health
	router.HandleFunc("GET /health", healthHandler)

	// Swagger
	router.Handle("/swagger/",
		httpSwagger.Handler(
			httpSwagger.URL("/swagger/doc.json"), //The url pointing to API definition
			httpSwagger.DeepLinking(true),
			httpSwagger.DocExpansion("none"),
			httpSwagger.DomID("swagger-ui"),
		),
	)

	log.Printf("listing on port: %d", config.Server.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Server.Port), muxWithMiddleware(router))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
