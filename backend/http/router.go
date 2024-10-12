package http

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
)

type modifyRequest struct {
	What  string   `json:"what"`  // Answer to: what data type?
	Which []string `json:"which"` // Answer to: which fields?
}

var (
	store     *storage.Storage
	server    *settings.Server
	fileCache FileCache
)

func SetupEnv(storage *storage.Storage, s *settings.Server, cache FileCache) {
	store = storage
	server = s
	fileCache = cache
}

func Setup(imgSvc ImgService, assetsFs fs.FS) {
	server.Clean()

	router := http.NewServeMux()
	port := 9989

	// Middleware for Content-Security-Policy
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
		http.NotFound(w, r)
	})

	// Static and index file handlers
	index, static := getStaticHandlers(store, server, assetsFs)
	router.Handle("/static/", http.StripPrefix("/static", static))
	router.Handle("/", index)

	// API group routing
	api := http.NewServeMux()
	router.Handle("/api/", http.StripPrefix("/api", api))

	// API routes
	api.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handle(loginHandler, "", store, server).ServeHTTP(w, r)
	})
	api.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		handle(signupHandler, "", store, server).ServeHTTP(w, r)
	})
	api.HandleFunc("/renew", func(w http.ResponseWriter, r *http.Request) {
		handle(renewHandler, "", store, server).ServeHTTP(w, r)
	})

	// Users routes
	api.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handle(usersGetHandler, "", store, server).ServeHTTP(w, r)
		case http.MethodPost:
			handle(userPostHandler, "", store, server).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Handle user-specific routes with ID
	api.HandleFunc("/users/{id:[0-9]+}", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			handle(userPutHandler, "", store, server).ServeHTTP(w, r)
		case http.MethodGet:
			handle(userGetHandler, "", store, server).ServeHTTP(w, r)
		case http.MethodDelete:
			handle(userDeleteHandler, "", store, server).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Resources routes
	api.HandleFunc("/resources", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handle(resourceGetHandler, "/api/resources", store, server).ServeHTTP(w, r)
		case http.MethodDelete:
			handle(resourceDeleteHandler(fileCache), "/api/resources", store, server).ServeHTTP(w, r)
		case http.MethodPost:
			handle(resourcePostHandler(fileCache), "/api/resources", store, server).ServeHTTP(w, r)
		case http.MethodPut:
			handle(resourcePutHandler, "/api/resources", store, server).ServeHTTP(w, r)
		case http.MethodPatch:
			handle(resourcePatchHandler(fileCache), "/api/resources", store, server).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Additional API routes
	api.HandleFunc("/usage", func(w http.ResponseWriter, r *http.Request) {
		handle(diskUsage, "/api/usage", store, server).ServeHTTP(w, r)
	})

	api.HandleFunc("/shares", func(w http.ResponseWriter, r *http.Request) {
		handle(shareListHandler, "/api/shares", store, server).ServeHTTP(w, r)
	})

	api.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handle(shareGetsHandler, "/api/share", store, server).ServeHTTP(w, r)
		case http.MethodPost:
			handle(sharePostHandler, "/api/share", store, server).ServeHTTP(w, r)
		case http.MethodDelete:
			handle(shareDeleteHandler, "/api/share", store, server).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Settings routes
	api.HandleFunc("/settings", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handle(settingsGetHandler, "", store, server).ServeHTTP(w, r)
		case http.MethodPut:
			handle(settingsPutHandler, "", store, server).ServeHTTP(w, r)
		default:
			http.NotFound(w, r)
		}
	})

	// Raw and Preview routes
	api.HandleFunc("/raw", func(w http.ResponseWriter, r *http.Request) {
		handle(rawHandler, "/api/raw", store, server).ServeHTTP(w, r)
	})

	api.HandleFunc("/preview/{size}/{path:.*}", func(w http.ResponseWriter, r *http.Request) {
		handle(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview", store, server).ServeHTTP(w, r)
	})

	// Search route
	api.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		handle(searchHandler, "/api/search", store, server).ServeHTTP(w, r)
	})

	// Public routes
	public := http.NewServeMux()
	api.Handle("/public/", http.StripPrefix("/public", public))

	public.HandleFunc("/publicUser", func(w http.ResponseWriter, r *http.Request) {
		handle(publicUserGetHandler, "", store, server).ServeHTTP(w, r)
	})

	public.HandleFunc("/dl", func(w http.ResponseWriter, r *http.Request) {
		handle(publicDlHandler, "/api/public/dl/", store, server).ServeHTTP(w, r)
	})

	public.HandleFunc("/share", func(w http.ResponseWriter, r *http.Request) {
		handle(publicShareHandler, "/api/public/share/", store, server).ServeHTTP(w, r)
	})

	log.Printf("listing on port: %d", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
