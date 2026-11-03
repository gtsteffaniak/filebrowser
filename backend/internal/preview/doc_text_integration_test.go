package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/folio"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

func TestFolioTextPath_utf8TruncatedPlaywrightFile(t *testing.T) {
	realPath := filepath.Join("..", "..", "..", "frontend", "tests", "playwright-files", "utf8-truncated.txt")
	if _, err := os.Stat(realPath); err != nil {
		t.Skip("playwright fixture missing")
	}
	temp := filepath.Join(t.TempDir(), "snippet.txt")
	path, cleanup, err := folioTextPath(realPath, temp, true)
	require.NoError(t, err)
	defer cleanup()
	require.Equal(t, temp, path)

	opts, err := folioRenderOptions("small")
	require.NoError(t, err)
	data, ok, err := folio.RenderPageJPEG(t.Context(), path, 0, opts)
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
	file := iteminfo.ExtendedFileInfo{
		RealPath: realPath,
	}
	file.Name = "utf8-truncated.txt"
	file.Type = "text/plain"

	tempPath := filepath.Join(s.cacheDir, "doc.txt")
	out, err := s.GenerateImageFromDoc(t.Context(), file, tempPath, 0, "small")
	require.NoError(t, err)
	require.NotEmpty(t, out)
}
