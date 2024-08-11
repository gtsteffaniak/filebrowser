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
		auther := raw.(*auth.JSONAuth)
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

func getStaticHandlers(store *storage.Storage, server *settings.Server, assetsFs fs.FS) (index, static http.Handler) {
	index = handle(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.Method != http.MethodGet {
			return http.StatusNotFound, nil
		}

		w.Header().Set("x-xss-protection", "1; mode=block")
		return handleWithStaticData(w, r, d, assetsFs, "public/index.html", "text/html; charset=utf-8")
	}, "", store, server)

	static = handle(func(w http.ResponseWriter, r *http.Request, d *data) (int, error) {
		if r.Method != http.MethodGet {
			return http.StatusNotFound, nil
		}

		const maxAge = 86400 // 1 day
		w.Header().Set("Cache-Control", fmt.Sprintf("public, max-age=%v", maxAge))

		if d.settings.Frontend.Files != "" {
			if strings.HasPrefix(r.URL.Path, "img/") {
				fPath := filepath.Join(d.settings.Frontend.Files, r.URL.Path)
				if _, err := os.Stat(fPath); err == nil {
					http.ServeFile(w, r, fPath)
					return 0, nil
				}
			} else if r.URL.Path == "custom.css" && d.settings.Frontend.Files != "" {
				http.ServeFile(w, r, filepath.Join(d.settings.Frontend.Files, "custom.css"))
				return 0, nil
			}
		}

		if !strings.HasSuffix(r.URL.Path, ".js") {
			http.FileServer(http.FS(assetsFs)).ServeHTTP(w, r)
			return 0, nil
		}

		fileContents, err := fs.ReadFile(assetsFs, r.URL.Path+".gz")
		if err != nil {
			return http.StatusNotFound, err
		}

		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8") // Set the correct MIME type for JavaScript files

		if _, err := w.Write(fileContents); err != nil {
			return http.StatusInternalServerError, err
		}

		return 0, nil
	}, "/static/", store, server)

	return index, static
}
