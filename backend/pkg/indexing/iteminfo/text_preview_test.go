package iteminfo

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTextPreviewKinds(t *testing.T) {
	require.Equal(t, "markdown", TextPreviewKind(".md"))
	require.Equal(t, "html", TextPreviewKind(".htm"))
	require.Equal(t, "html", TextPreviewKind(".html"))
	require.Equal(t, "css", TextPreviewKind(".css"))
	require.Equal(t, "rtf", TextPreviewKind(".rtf"))
	require.Equal(t, "plain", TextPreviewKind(".txt"))
	require.Equal(t, "csv", TextPreviewKind(".csv"))
	require.Equal(t, "csv", TextPreviewKind(".tsv"))
	require.Equal(t, "code", TextPreviewKind(".go"))
	require.Equal(t, "", TextPreviewKind(".svg"), "svg stays on the image path")
	require.Equal(t, "", TextPreviewKind(".pdf"))
}

func TestIsTextPreviewable(t *testing.T) {
	for _, mt := range []string{"text/plain", "text/markdown", "text/lyrics", "text/x-go", "text/html"} {
		require.True(t, IsTextPreviewable(mt), mt)
	}
	for _, mt := range []string{"application/json", "application/yaml", "application/sql", "application/x-sh", "application/x-php", "application/x-powershell", "application/x-tex", "application/rtf", "text/rtf"} {
		require.True(t, IsTextPreviewable(mt), mt)
	}
	// Mimetypes shared with binaries (.exe, .bat via application/x-msdownload) must stay non-previewable.
	for _, mt := range []string{"", "image/svg+xml", "application/pdf", "application/x-sqlite3", "application/postscript", "application/backup", "application/wasm", "application/x-msdownload", "application/x-msdos-program", "application/octet-stream"} {
		require.False(t, IsTextPreviewable(mt), mt)
	}
}

func TestTextPreviewRenderKind(t *testing.T) {
	require.Equal(t, "markdown", TextPreviewRenderKind("notes.md"))
	require.Equal(t, "plain", TextPreviewRenderKind("50. Holy Water.lrc"))
	require.Equal(t, "plain", TextPreviewRenderKind("cert.pem"))
}

func TestTextPreviewMimeTypesIsSortedAndMatchesAllowlist(t *testing.T) {
	types := TextPreviewMimeTypes()
	require.NotEmpty(t, types)
	require.True(t, sort.StringsAreSorted(types), "list must be sorted for a stable globalVars value")
	for _, mt := range types {
		require.True(t, IsTextPreviewable(mt), mt)
	}
	require.Contains(t, types, "application/json")
}
