package web

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/swaggo/swag"
)

// swaggerUIAssets holds only the swagger-ui files the index page needs.
// The full swaggo/files bundle (ES bundles, legacy swagger-ui.js and source maps)
// is intentionally not embedded to keep the binary small.
// Copied from the swagger-ui-dist bundle shipped with github.com/swaggo/files v1.0.1.
// To upgrade, replace these files from a newer swagger-ui-dist release.
//
//go:embed swaggerui
var swaggerUIAssets embed.FS

var swaggerUIFiles, _ = fs.Sub(swaggerUIAssets, "swaggerui")

var swaggerIndexTmpl = template.Must(template.ParseFS(swaggerUIAssets, "swaggerui/index.html.tmpl"))

// swaggerStaticFiles is the allowlist of assets served from the embedded FS.
var swaggerStaticFiles = map[string]bool{
	"swagger-ui-bundle.js":            true,
	"swagger-ui-standalone-preset.js": true,
	"swagger-ui.css":                  true,
	"favicon-16x16.png":               true,
	"favicon-32x32.png":               true,
}

type swaggerIndexData struct {
	URL                  string
	DocExpansion         string
	DomID                string
	DeepLinking          bool
	PersistAuthorization bool
}

func swaggerHandler(w http.ResponseWriter, r *http.Request, d *requestContext) (int, error) {
	if !d.User.Permissions.Api {
		return http.StatusForbidden, nil
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		return http.StatusMethodNotAllowed, nil
	}

	name := path.Base(r.URL.Path)
	switch {
	case name == "index.html" || strings.HasSuffix(r.URL.Path, "/"):
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		data := swaggerIndexData{
			URL:          settings.Config.Http.BaseURL + "swagger/doc.json",
			DeepLinking:  true,
			DocExpansion: "none",
			DomID:        "swagger-ui",
		}
		if err := swaggerIndexTmpl.Execute(w, data); err != nil {
			return http.StatusInternalServerError, err
		}
	case name == "doc.json":
		doc, err := swag.ReadDoc(swag.Name)
		if err != nil {
			return http.StatusInternalServerError, err
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write([]byte(doc))
	case swaggerStaticFiles[name]:
		http.ServeFileFS(w, r, swaggerUIFiles, name)
	default:
		return http.StatusNotFound, nil
	}
	return http.StatusOK, nil
}
