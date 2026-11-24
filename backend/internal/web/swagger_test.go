package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/filebrowser/backend/swagger/docs"
	"github.com/swaggo/swag"
)

func TestSwaggerHandler(t *testing.T) {
	orig := settings.Config.Http.BaseURL
	settings.Config.Http.BaseURL = "/fb/"
	t.Cleanup(func() { settings.Config.Http.BaseURL = orig })
	swag.Register(docs.SwaggerInfo.InstanceName(), docs.SwaggerInfo)

	apiCtx := &requestContext{User: &users.User{FrontendUser: users.FrontendUser{Permissions: users.Permissions{Api: true}}}}
	noApiCtx := &requestContext{User: &users.User{}}

	serve := func(t *testing.T, method, target string, d *requestContext) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		status, err := swaggerHandler(rec, httptest.NewRequest(method, target, nil), d)
		if err != nil {
			t.Fatalf("handler error: %v", err)
		}
		if status != http.StatusOK {
			rec.WriteHeader(status)
		}
		return rec
	}

	t.Run("forbidden without api permission", func(t *testing.T) {
		if rec := serve(t, http.MethodGet, "/fb/swagger/index.html", noApiCtx); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})

	t.Run("method not allowed", func(t *testing.T) {
		if rec := serve(t, http.MethodPost, "/fb/swagger/index.html", apiCtx); rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("got %d, want 405", rec.Code)
		}
	})

	t.Run("index page points at base url doc.json and embedded bundle", func(t *testing.T) {
		rec := serve(t, http.MethodGet, "/fb/swagger/index.html", apiCtx)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		body := rec.Body.String()
		// html/template escapes "/" inside JS strings as "\/", which is valid JavaScript.
		for _, want := range []string{`url: "\/fb\/swagger\/doc.json"`, "swagger-ui-bundle.js", "swagger-ui-standalone-preset.js"} {
			if !strings.Contains(body, want) {
				t.Errorf("index page missing %q", want)
			}
		}
	})

	t.Run("allowlisted static asset is served", func(t *testing.T) {
		rec := serve(t, http.MethodGet, "/fb/swagger/swagger-ui.css", apiCtx)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
			t.Errorf("content-type %q, want text/css", ct)
		}
	})

	t.Run("dropped swagger-ui bundles are not served", func(t *testing.T) {
		for _, name := range []string{"swagger-ui.js", "swagger-ui-es-bundle.js", "swagger-ui-bundle.js.map", "index.html.tmpl"} {
			if rec := serve(t, http.MethodGet, "/fb/swagger/"+name, apiCtx); rec.Code != http.StatusNotFound {
				t.Errorf("%s: got %d, want 404", name, rec.Code)
			}
		}
	})

	t.Run("doc.json is the registered spec", func(t *testing.T) {
		rec := serve(t, http.MethodGet, "/fb/swagger/doc.json", apiCtx)
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), `"swagger"`) {
			t.Errorf("doc.json does not look like a swagger spec")
		}
	})
}
