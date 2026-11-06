package preview

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenderDocPageJPEG_txt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hello.txt")
	require.NoError(t, os.WriteFile(path, []byte("Hello folio preview"), 0o644))

	data, ok, err := renderDocPageJPEG(t.Context(), path, 0, docRenderOptions{
		MaxWidth:    256,
		MaxHeight:   256,
		Fill:        true,
		JpegQuality: 80,
	}, "hello.txt", "text/plain")
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEmpty(t, data)
	require.Equal(t, byte(0xff), data[0])
	require.Equal(t, byte(0xd8), data[1])
}

func TestRenderDocPageJPEG_unsupportedExtension(t *testing.T) {
	data, ok, err := renderDocPageJPEG(t.Context(), "/nonexistent/file.xyz", 0, docRenderOptions{}, "file.xyz", "application/octet-stream")
	require.NoError(t, err)
	require.False(t, ok)
	require.Nil(t, data)
}
