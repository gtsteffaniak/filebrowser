package http

import (
	"io/fs"
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
	mux := http.NewServeMux()
	// Middleware to set Content-Security-Policy
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", `default-src 'self'; style-src 'unsafe-inline';`)
	})
	// Health endpoint
	mux.HandleFunc("/health", healthHandler)

	// API endpoints
	api := mux.PathPrefix("/api").SubMux()
	api.Handle("/login", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		loginHandler(w, r, store, server)
	})).Methods("POST")
	api.Handle("/signup", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		signupHandler(w, r, store, server)
	})).Methods("POST")
	api.Handle("/renew", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		renewHandler(w, r, store, server)
	})).Methods("POST")

	// Users endpoints
	users := api.PathPrefix("/users").SubMux()
	users.Handle("/", http.HandlerFunc(usersGetHandler)).Methods("GET")
	users.Handle("/", http.HandlerFunc(userPostHandler)).Methods("POST")
	users.Handle("/{id:[0-9]+}", http.HandlerFunc(userPutHandler)).Methods("PUT")
	users.Handle("/{id:[0-9]+}", http.HandlerFunc(userGetHandler)).Methods("GET")
	users.Handle("/{id:[0-9]+}", http.HandlerFunc(userDeleteHandler)).Methods("DELETE")
	// Static file server
	index, static := getStaticHandlers(store, server, assetsFs)
	mux.Handle("/static/", http.StripPrefix("/static/", static))
	mux.Handle("/", index)
	// Resources endpoints
	api.Handle("/resources", http.HandlerFunc(resourceGetHandler)).Methods("GET")
	api.Handle("/resources", http.HandlerFunc(resourceDeleteHandler(fileCache))).Methods("DELETE")
	api.Handle("/resources", http.HandlerFunc(resourcePostHandler(fileCache))).Methods("POST")
	api.Handle("/resources", http.HandlerFunc(resourcePutHandler)).Methods("PUT")
	api.Handle("/resources", http.HandlerFunc(resourcePatchHandler(fileCache))).Methods("PATCH")

	// Usage endpoint
	api.Handle("/usage", http.HandlerFunc(diskUsage)).Methods("GET")

	// Shares endpoints
	api.Handle("/shares", http.HandlerFunc(shareListHandler)).Methods("GET")
	api.Handle("/share", http.HandlerFunc(shareGetsHandler)).Methods("GET")
	api.Handle("/share", http.HandlerFunc(sharePostHandler)).Methods("POST")
	api.Handle("/share", http.HandlerFunc(shareDeleteHandler)).Methods("DELETE")

	// Settings endpoints
	api.Handle("/settings", http.HandlerFunc(settingsGetHandler)).Methods("GET")
	api.Handle("/settings", http.HandlerFunc(settingsPutHandler)).Methods("PUT")

	// Raw endpoint
	api.Handle("/raw", http.HandlerFunc(rawHandler)).Methods("GET")

	// Preview endpoint
	api.Handle("/preview/{size}/{path:.*}", http.HandlerFunc(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview))).Methods("GET")

	// Search endpoint
	api.Handle("/search", http.HandlerFunc(searchHandler)).Methods("GET")

	// Public endpoints
	public := api.PathPrefix("/public").SubMux()
	public.Handle("/publicUser", http.HandlerFunc(publicUserGetHandler)).Methods("GET")
	public.Handle("/dl/", http.HandlerFunc(publicDlHandler)).Methods("GET")
	public.Handle("/share/", http.HandlerFunc(publicShareHandler)).Methods("GET")

	return mux, nil
}

func NewHandler() (http.Handler, error) {
	// NOTE: This fixes the issue where it would redirect if people did not put a
	// trailing slash in the end. I hate this decision since this allows some awful
	// URLs https://www.gorillatoolkit.org/pkg/mux#Router.SkipClean
	r = r.SkipClean(true)
	monkey := func(fn handleFunc, prefix string) http.Handler {
		return handle(fn, prefix, store, server)
	}
	r.HandleFunc("/health", healthHandler)
	r.PathPrefix("/static").Handler(static)
	r.NotFoundHandler = index
	api := r.PathPrefix("/api").Subrouter()
	api.Handle("/login", monkey(loginHandler, ""))
	api.Handle("/signup", monkey(signupHandler, ""))
	api.Handle("/renew", monkey(renewHandler, ""))
	users := api.PathPrefix("/users").Subrouter()
	users.Handle("", monkey(usersGetHandler, "")).Methods("GET")
	users.Handle("", monkey(userPostHandler, "")).Methods("POST")
	users.Handle("/{id:[0-9]+}", monkey(userPutHandler, "")).Methods("PUT")
	users.Handle("/{id:[0-9]+}", monkey(userGetHandler, "")).Methods("GET")
	users.Handle("/{id:[0-9]+}", monkey(userDeleteHandler, "")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourceGetHandler, "/api/resources")).Methods("GET")
	api.PathPrefix("/resources").Handler(monkey(resourceDeleteHandler(fileCache), "/api/resources")).Methods("DELETE")
	api.PathPrefix("/resources").Handler(monkey(resourcePostHandler(fileCache), "/api/resources")).Methods("POST")
	api.PathPrefix("/resources").Handler(monkey(resourcePutHandler, "/api/resources")).Methods("PUT")
	api.PathPrefix("/resources").Handler(monkey(resourcePatchHandler(fileCache), "/api/resources")).Methods("PATCH")
	api.PathPrefix("/usage").Handler(monkey(diskUsage, "/api/usage")).Methods("GET")
	api.Path("/shares").Handler(monkey(shareListHandler, "/api/shares")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(shareGetsHandler, "/api/share")).Methods("GET")
	api.PathPrefix("/share").Handler(monkey(sharePostHandler, "/api/share")).Methods("POST")
	api.PathPrefix("/share").Handler(monkey(shareDeleteHandler, "/api/share")).Methods("DELETE")
	api.Handle("/settings", monkey(settingsGetHandler, "")).Methods("GET")
	api.Handle("/settings", monkey(settingsPutHandler, "")).Methods("PUT")
	api.PathPrefix("/raw").Handler(monkey(rawHandler, "/api/raw")).Methods("GET")
	api.PathPrefix("/preview/{size}/{path:.*}").
		Handler(monkey(previewHandler(imgSvc, fileCache, server.EnableThumbnails, server.ResizePreview), "/api/preview")).Methods("GET")
	api.PathPrefix("/search").Handler(monkey(searchHandler, "/api/search")).Methods("GET")
	public := api.PathPrefix("/public").Subrouter()
	public.Handle("/publicUser", monkey(publicUserGetHandler, "")).Methods("GET")
	public.PathPrefix("/dl").Handler(monkey(publicDlHandler, "/api/public/dl/")).Methods("GET")
	public.PathPrefix("/share").Handler(monkey(publicShareHandler, "/api/public/share/")).Methods("GET")
	return stripPrefix(server.BaseURL, r), nil
}
