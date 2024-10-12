package http

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/gtsteffaniak/filebrowser/auth"
	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
	"github.com/gtsteffaniak/filebrowser/version"
)

func handleWithStaticData(w http.ResponseWriter, _ *http.Request, d *data, fSys fs.FS, file, contentType string) (int, error) {
	w.Header().Set("Content-Type", contentType)

	auther, err := d.store.Auth.Get(d.settings.Auth.Method)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	data := map[string]interface{}{
		"Name":                  d.settings.Frontend.Name,
		"DisableExternal":       d.settings.Frontend.DisableExternal,
		"DisableUsedPercentage": d.settings.Frontend.DisableUsedPercentage,
		"darkMode":              settings.Config.UserDefaults.DarkMode,
		"Color":                 d.settings.Frontend.Color,
		"BaseURL":               d.server.BaseURL,
		"Version":               version.Version,
		"CommitSHA":             version.CommitSHA,
		"StaticURL":             path.Join(d.server.BaseURL, "/static"),
		"Signup":                settings.Config.Auth.Signup,
		"NoAuth":                d.settings.Auth.Method == "noauth",
		"AuthMethod":            d.settings.Auth.Method,
		"LoginPage":             auther.LoginPage(),
		"CSS":                   false,
		"ReCaptcha":             false,
		"EnableThumbs":          d.server.EnableThumbnails,
		"ResizePreview":         d.server.ResizePreview,
		"EnableExec":            d.server.EnableExec,
	}

	if d.settings.Frontend.Files != "" {
		fPath := filepath.Join(d.settings.Frontend.Files, "custom.css")
		_, err := os.Stat(fPath) //nolint:govet

		if err != nil && !os.IsNotExist(err) {
			log.Printf("couldn't load custom styles: %v", err)
		}

		if err == nil {
			data["CSS"] = true
		}
	}

	if d.settings.Auth.Method == "password" {
		raw, err := d.store.Auth.Get(d.settings.Auth.Method) //nolint:govet
		if err != nil {
			return http.StatusInternalServerError, err
		}

		auther, ok := raw.(*auth.JSONAuth)
		if !ok {
			return http.StatusInternalServerError, fmt.Errorf("failed to assert type *auth.JSONAuth")
		}

		if auther.ReCaptcha != nil {
			data["ReCaptcha"] = auther.ReCaptcha.Key != "" && auther.ReCaptcha.Secret != ""
			data["ReCaptchaHost"] = auther.ReCaptcha.Host
			data["ReCaptchaKey"] = auther.ReCaptcha.Key
		}
	}

	b, err := json.Marshal(data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	data["Json"] = strings.ReplaceAll(string(b), `'`, `\'`)

	fileContents, err := fs.ReadFile(fSys, file)
	if err != nil {
		if err == os.ErrNotExist {
			return http.StatusNotFound, err
		}
		return http.StatusInternalServerError, err
	}
	index := template.Must(template.New("index").Delims("[{[", "]}]").Parse(string(fileContents)))
	err = index.Execute(w, data)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return 0, nil
}

package http

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/settings"
	"github.com/gtsteffaniak/filebrowser/storage"
)

func getStaticHandlers(store *storage.Storage, server *settings.Server, assetsFs fs.FS) (http.Handler, http.Handler) {
	// Index handler
	index := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		w.Header().Set("x-xss-protection", "1; mode=block")

		// Serve the index.html file from the embedded filesystem
		file, err := assetsFs.Open("public/index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer file.Close()

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", fs.ModTime(file), file)
	})

	// Static file handler
	static := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}

		const maxAge = 86400 // 1 day cache
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))

		// Serve custom static files if configured in settings
		if store != nil && server != nil && server.Frontend.Files != "" {
			filePath := filepath.Join(server.Frontend.Files, r.URL.Path)
			if strings.HasPrefix(r.URL.Path, "/img/") {
				// Serve image files
				if _, err := os.Stat(filePath); err == nil {
					http.ServeFile(w, r, filePath)
					return
				}
			} else if r.URL.Path == "/custom.css" {
				// Serve custom CSS file
				http.ServeFile(w, r, filepath.Join(server.Frontend.Files, "custom.css"))
				return
			}
		}

		// Serve JavaScript files with gzip encoding if available
		if strings.HasSuffix(r.URL.Path, ".js") {
			gzFile := r.URL.Path + ".gz"
			fileContents, err := fs.ReadFile(assetsFs, gzFile)
			if err != nil {
				http.NotFound(w, r)
				return
			}

			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			w.Write(fileContents)
			return
		}

		// Serve other static files from embedded filesystem
		http.FileServer(http.FS(assetsFs)).ServeHTTP(w, r)
	})

	return index, static
}

