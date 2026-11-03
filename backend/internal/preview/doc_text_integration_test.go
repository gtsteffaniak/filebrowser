package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/folio"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

func TestFolioRender_utf8TruncatedPlaywrightFile(t *testing.T) {
	realPath := filepath.Join("..", "..", "..", "frontend", "tests", "playwright-files", "utf8-truncated.txt")
	if _, err := os.Stat(realPath); err != nil {
		t.Skip("playwright fixture missing")
	}
	opts, err := folioRenderOptions("small")
	require.NoError(t, err)
	data, ok, err := folio.RenderPageJPEG(t.Context(), realPath, 0, opts, "utf8-truncated.txt", "text/plain")
	require.NoError(t, err)
	require.True(t, ok, "err=%v", err)
	require.NotEmpty(t, data)
}

func TestFolioRender_cssAsTxt(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "styles.css")
	require.NoError(t, os.WriteFile(cssPath, []byte("body { color: red; }\n"), 0o644))
	opts, err := folioRenderOptions("small")
	require.NoError(t, err)
	data, ok, err := folio.RenderPageJPEG(t.Context(), cssPath, 0, opts, "styles.css", "text/css")
	require.NoError(t, err)
	require.True(t, ok, "err=%v", err)
	require.NotEmpty(t, data)
}

func TestGenerateImageFromDoc_utf8TruncatedPlaywrightFile(t *testing.T) {
	realPath := filepath.Join("..", "..", "..", "frontend", "tests", "playwright-files", "utf8-truncated.txt")
	if _, err := os.Stat(realPath); err != nil {
		t.Skip("playwright fixture missing")
	}
	s := &Service{cacheDir: t.TempDir()}
	file := iteminfo.ExtendedFileInfo{RealPath: realPath}
	file.Name = "utf8-truncated.txt"
	file.Type = "text/plain"

	out, err := s.GenerateImageFromDoc(t.Context(), file, 0, "small")
	require.NoError(t, err)
	require.NotEmpty(t, out)
}
