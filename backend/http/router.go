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
	imgSvc    ImgService
)

func SetupEnv(storage *storage.Storage, s *settings.Server, cache FileCache, imgService ImgService) {
	store = storage
	server = s
	fileCache = cache
	imgSvc = imgService
}

func Setup(assetsFs fs.FS) {
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
	api.HandleFunc("GET /login", withUser(loginHandler))
	api.HandleFunc("GET /signup", withUser(signupHandler))
	api.HandleFunc("GET /renew", withUser(renewHandler))

	api.HandleFunc("GET /users", withSelfOrAdmin(usersGetHandler))
	api.HandleFunc("POST /users", withSelfOrAdmin(userPostHandler))

	api.HandleFunc("GET /users/{id:[0-9]+}", withSelfOrAdmin(userGetHandler))
	api.HandleFunc("PUT /users/{id:[0-9]+}", withSelfOrAdmin(userPutHandler))
	api.HandleFunc("POST /users/{id:[0-9]+}", withSelfOrAdmin(userPostHandler))
	api.HandleFunc("DELETE /users/{id:[0-9]+}", withSelfOrAdmin(userDeleteHandler))

	// Resources routes
	api.HandleFunc("GET /resources", withUser(resourceGetHandler))
	api.HandleFunc("DELETE /resources", withUser(resourceDeleteHandler))
	api.HandleFunc("POST /resources", withUser(resourcePostHandler))
	api.HandleFunc("PUT /resources", withUser(resourcePutHandler))
	api.HandleFunc("PATCH /resources", withUser(resourcePatchHandler))

	// Additional API routes
	api.HandleFunc("GET /usage", withUser(diskUsage))

	api.HandleFunc("GET /shares", withUser(shareListHandler))

	api.HandleFunc("GET /share", withUser(shareGetsHandler))
	api.HandleFunc("POST /share", withUser(sharePostHandler))
	api.HandleFunc("DELETE /share", withUser(shareDeleteHandler))

	api.HandleFunc("GET /settings", withAdmin(settingsGetHandler))
	api.HandleFunc("PUT /settings", withUser(settingsPutHandler))

	api.HandleFunc("GET /raw", withUser(rawHandler))

	api.HandleFunc("GET /preview/{size}/{path:.*}", withUser(previewHandler))

	api.HandleFunc("GET /search", withUser(searchHandler))

	// Public routes
	public := http.NewServeMux()
	public.Handle("/public/", http.StripPrefix("/api/public", public))

	public.HandleFunc("GET /publicUser", withUser(publicUserGetHandler))
	public.HandleFunc("GET /publicUser", withUser(publicUserGetHandler))

	public.HandleFunc("GET /dl", withUser(publicDlHandler))

	public.HandleFunc("GET /share", withUser(publicShareHandler))

	log.Printf("listing on port: %d", port)
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), router)
	if err != nil {
		log.Fatalf("could not start server: %v", err)
	}
}
