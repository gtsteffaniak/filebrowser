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

	api.HandleFunc("GET /users", withAdmin(usersGetHandler))
	api.HandleFunc("POST /users", withAdmin(usersPostHandler))
	api.HandleFunc("GET /users/{id}", withSelfOrAdmin(userGetHandler))
	api.HandleFunc("PUT /users/{id}", withSelfOrAdmin(userPutHandler))
	//api.HandleFunc("POST /users/{id}", withSelfOrAdmin(userPostHandler))
	api.HandleFunc("DELETE /users/{id}", withSelfOrAdmin(userDeleteHandler))

	// API routes
	api.HandleFunc("POST /login", loginHandler)
	api.HandleFunc("GET /signup", withUser(signupHandler))
	api.HandleFunc("POST /renew", withUser(renewHandler))
	// Resources routes
	api.HandleFunc("GET /resources/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/resources/", withUser(resourceGetHandler)).ServeHTTP(w, r)
	})
	api.HandleFunc("DELETE /resources/", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources/", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources/", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resource/", withUser(resourcePatchHandler))

	// Additional API routes
	api.HandleFunc("GET /usage/", func(w http.ResponseWriter, r *http.Request) {
		http.StripPrefix("/usage/", withUser(diskUsage)).ServeHTTP(w, r)
	})

	api.HandleFunc("GET /shares", withPermShare(shareListHandler))

	api.HandleFunc("GET /share/", withPermShare(shareGetsHandler))
	api.HandleFunc("POST /share/", withPermShare(sharePostHandler))
	api.HandleFunc("DELETE /share/", withPermShare(shareDeleteHandler))

	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withUser(settingsPutHandler))

	api.HandleFunc("GET /raw", withUser(rawHandler))

	api.HandleFunc("GET /preview/{size}/{path...}", withUser(previewHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))

	api.HandleFunc("GET /public/publicUser", withUser(publicUserGetHandler))
	api.HandleFunc("GET /public/dl", withHashFile(publicDlHandler))
	api.HandleFunc("GET /public/share/", withHashFile(publicShareHandler))

	router.Handle("/api/", http.StripPrefix("/api", api))

	// Static and index file handlers
	router.HandleFunc("GET /static/", staticFilesHandler)
	router.HandleFunc("/", indexHandler)

	// health
	router.HandleFunc("GET /health", healthHandler)

	log.Printf("listing on port: %d", config.Server.Port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", config.Server.Port), muxWithMiddleware(router))
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
