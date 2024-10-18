package http

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"text/template"
	"time"

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

	api.HandleFunc("GET /users", withSelfOrAdmin(usersGetHandler))
	api.HandleFunc("POST /users", withSelfOrAdmin(userPostHandler))
	api.HandleFunc("GET /users/{id}", withSelfOrAdmin(userGetHandler))
	api.HandleFunc("PUT /users/{id}", withSelfOrAdmin(userPutHandler))
	api.HandleFunc("POST /users/{id}", withSelfOrAdmin(userPostHandler))
	api.HandleFunc("DELETE /users/{id}", withSelfOrAdmin(userDeleteHandler))

	// API routes
	api.HandleFunc("POST /login", loginHandler)
	api.HandleFunc("GET /signup", withUser(signupHandler))
	api.HandleFunc("POST /renew", withUser(renewHandler))
	// Resources routes
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(resourcePatchHandler))

	// Additional API routes
	api.HandleFunc("GET /usage", withUser(diskUsage))

	api.HandleFunc("GET /shares", withPermShare(shareListHandler))

	api.HandleFunc("GET /share", withPermShare(shareGetsHandler))
	api.HandleFunc("POST /share", withPermShare(sharePostHandler))
	api.HandleFunc("DELETE /share", withPermShare(shareDeleteHandler))

	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withUser(settingsPutHandler))

	api.HandleFunc("GET /raw", withUser(rawHandler))

	api.HandleFunc("GET /preview/{size}/{path}", withUser(previewHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))
	router.Handle("/api/", http.StripPrefix("/api", api))
	// Public routes
	public := http.NewServeMux()
	public.HandleFunc("GET /publicUser", withUser(publicUserGetHandler))
	public.HandleFunc("GET /dl", withHashFile(publicDlHandler))
	public.HandleFunc("GET /share", withHashFile(publicShareHandler))
	public.Handle("/api/public/", http.StripPrefix("/api/public", public))

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

func muxWithMiddleware(mux *http.ServeMux) *http.ServeMux {
	wrappedMux := http.NewServeMux()
	wrappedMux.Handle("/", LoggingMiddleware(mux))
	return wrappedMux
}

// ResponseWriterWrapper wraps the standard http.ResponseWriter to capture the status code
type ResponseWriterWrapper struct {
	http.ResponseWriter
	StatusCode int
}

// LoggingMiddleware logs each request with consistent spacing and colorized output
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the ResponseWriter to capture the status code
		wrappedWriter := &ResponseWriterWrapper{ResponseWriter: w, StatusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(wrappedWriter, r)

		// Determine the color based on the status code
		color := "\033[32m" // Default green color
		if wrappedWriter.StatusCode >= 400 && wrappedWriter.StatusCode < 500 {
			color = "\033[33m" // Yellow for client errors (4xx)
		} else if wrappedWriter.StatusCode >= 500 {
			color = "\033[31m" // Red for server errors (5xx)
		}

		// Reset color
		reset := "\033[0m"

		// Log the request details with consistent spacing and colorization
		log.Printf("%s%-7s | %3d | %-15s | %-12s | \"%s\"%s",
			color,                      // Start color
			r.Method,                   // HTTP method
			wrappedWriter.StatusCode,   // Status code
			r.RemoteAddr,               // Remote IP address
			time.Since(start).String(), // Time taken to process the request
			r.URL.Path,                 // URL path
			reset,                      // Reset color
		)
	})
}
