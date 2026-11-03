package folio

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestRepairFolioTextValidationWindow_playwrightFixture(t *testing.T) {
	realPath := filepath.Join("..", "..", "..", "frontend", "tests", "playwright-files", "utf8-truncated.txt")
	b, err := os.ReadFile(realPath)
	if err != nil {
		t.Skip("playwright fixture missing")
	}
	require.True(t, utf8.Valid(b))
	require.False(t, utf8.Valid(b[:folioTextCheckWindow]))

	fixed := repairFolioTextValidationWindow(b)
	require.True(t, utf8.Valid(fixed[:min(len(fixed), folioTextCheckWindow)]))
}

func TestOpenDocument_cssRealPath(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "styles.css")
	require.NoError(t, os.WriteFile(cssPath, []byte("body { color: red; }\n"), 0o644))

	d, err := openDocument(cssPath, "styles.css", "text/css")
	require.NoError(t, err)
	require.NoError(t, d.Close())
}

func TestReadTextPreviewPrefix_cap(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.log")
	payload := strings.Repeat("x", maxTextPreviewLoad+1024)
	require.NoError(t, os.WriteFile(path, []byte(payload), 0o644))

	b, err := readTextPreviewPrefix(path, maxTextPreviewLoad)
	require.NoError(t, err)
	require.Len(t, b, maxTextPreviewLoad)
}

func TestUseTextPreviewPrefix(t *testing.T) {
	require.True(t, useTextPreviewPrefix("styles.css", "text/css"))
	require.True(t, useTextPreviewPrefix("readme.txt", "text/plain"))
	require.False(t, useTextPreviewPrefix("page.html", "text/html"))
	require.False(t, useTextPreviewPrefix("doc.pdf", "application/pdf"))
}
