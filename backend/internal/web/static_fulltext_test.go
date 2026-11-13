package web

import (
	"encoding/json"
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The advanced-search "Search file contents" toggle is gated on this flag, so the
// page must learn the server's fullTextSearch setting from the injected globalVars
// instead of discovering it through a 400 after the user already searched.
func TestIndexGlobalVarsExposeContentSearchAvailability(t *testing.T) {
	originalConfig := settings.Config
	originalRenderer := templateRenderer
	originalAssets := assetFs
	t.Cleanup(func() {
		settings.Config = originalConfig
		templateRenderer = originalRenderer
		assetFs = originalAssets
	})

	settings.Config = settings.SetDefaults(true)
	// handleWithStaticData tolerates a missing loadingSpinners.css; an empty asset
	// filesystem keeps the test independent of the built frontend bundle.
	assetFs = fstest.MapFS{}
	templateRenderer = &TemplateRenderer{
		// Mirror the real page: inside a <script> element html/template emits
		// template.JS verbatim; in plain HTML context it entity-escapes the JSON.
		templates: template.Must(template.New("index.html").Parse(
			`<script id="app-config" type="application/json">{{ .globalVars }}</script>`)),
	}

	for _, enabled := range []bool{false, true} {
		settings.Config.Server.IndexSqlConfig.FullTextSearch = enabled
		req := httptest.NewRequest("GET", "/", nil)
		rec := httptest.NewRecorder()
		status, err := handleWithStaticData(rec, req, &requestContext{}, "index.html", "text/html")
		require.NoError(t, err)
		require.Equal(t, 200, status)

		body := rec.Body.String()
		const open = `<script id="app-config" type="application/json">`
		const close = `</script>`
		require.True(t, strings.HasPrefix(body, open) && strings.HasSuffix(body, close))
		var vars map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(body[len(open):len(body)-len(close)]), &vars))
		assert.Equal(t, enabled, vars["contentSearchAvailable"],
			"globalVars.contentSearchAvailable must mirror server.indexSqlConfig.fullTextSearch")
	}
}
