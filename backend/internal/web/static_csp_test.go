package web

import (
	"html/template"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newCSPTestRenderer() *TemplateRenderer {
	tmpl := template.Must(template.New("index.html").Parse(
		`<html><head><script nonce="{{ .cspNonce }}">x</script></head></html>`,
	))
	return &TemplateRenderer{templates: tmpl}
}

func TestRenderedHTMLContainsNonce(t *testing.T) {
	// Documents the html/template behavior that motivated the RawURLEncoding
	// switch: '+' inside a quoted attribute is escaped as &#43;, so a
	// StdEncoding nonce containing '+' never byte-matches the
	// 'nonce-...' CSP header source. '/' and '=' are preserved verbatim.
	plusCases := []string{"abc+def/ghi==", "a+b/c=="}
	for _, nonce := range plusCases {
		data := map[string]interface{}{"cspNonce": nonce}
		rec := httptest.NewRecorder()
		require.NoError(t, newCSPTestRenderer().Render(rec, "index.html", data))
		assert.NotContains(t, rec.Body.String(), `nonce="`+nonce+`"`,
			"html/template escapes '+' as &#43;, breaking the CSP match")
		assert.Contains(t, rec.Body.String(), "&#43;")
		assert.Contains(t, rec.Header().Get("Content-Security-Policy"), `'nonce-`+nonce+`'`)
	}

	verbatimCases := []string{"////====", "ABCDEFGHIJKLMNOPQRSTUV"}
	for _, nonce := range verbatimCases {
		data := map[string]interface{}{"cspNonce": nonce}
		rec := httptest.NewRecorder()
		require.NoError(t, newCSPTestRenderer().Render(rec, "index.html", data))
		assert.Contains(t, rec.Body.String(), `nonce="`+nonce+`"`)
		assert.Contains(t, rec.Header().Get("Content-Security-Policy"), `'nonce-`+nonce+`'`)
	}
}

func TestRenderedHTMLContainsGeneratedNonce(t *testing.T) {
	for i := 0; i < 50; i++ {
		data := map[string]interface{}{}
		rec := httptest.NewRecorder()
		require.NoError(t, newCSPTestRenderer().Render(rec, "index.html", data))
		nonce, ok := data["cspNonce"].(string)
		require.True(t, ok && nonce != "")
		assert.NotContains(t, nonce, "+")
		assert.NotContains(t, nonce, "/")
		assert.NotContains(t, nonce, "=")
		assert.Contains(t, rec.Body.String(), `nonce="`+nonce+`"`)
		assert.Contains(t, rec.Header().Get("Content-Security-Policy"), `'nonce-`+nonce+`'`)
		assert.True(t, strings.HasPrefix(rec.Header().Get("Content-Security-Policy"), "script-src 'self' 'nonce-"))
	}
}
