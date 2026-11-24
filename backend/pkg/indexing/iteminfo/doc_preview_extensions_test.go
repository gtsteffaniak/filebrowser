package iteminfo

import (
	"slices"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

func TestDocPreviewExtensionsForSPA_officeOnlyExtensions(t *testing.T) {
	t.Cleanup(func() {
		settings.Config.Integrations.OnlyOffice.Url = ""
	})

	settings.Config.Integrations.OnlyOffice.Url = ""
	exts := DocPreviewExtensionsForSPA()
	require.Contains(t, exts, "pdf")
	require.NotContains(t, exts, "hwp")
	require.NotContains(t, exts, "hwpx")

	settings.Config.Integrations.OnlyOffice.Url = "https://office.example"
	exts = DocPreviewExtensionsForSPA()
	require.Contains(t, exts, "hwp")
	require.Contains(t, exts, "hwpx")
}

func TestHasDocConvertableExtension_textAndFolio(t *testing.T) {
	t.Cleanup(func() {
		settings.Config.Integrations.Media.AlwaysUseOnlyOffice = false
	})

	require.True(t, HasDocConvertableExtension("readme.txt", "text/plain"))
	require.True(t, HasDocConvertableExtension("styles.css", "text/css"))
	require.True(t, HasDocConvertableExtension("doc.pdf", "application/pdf"))
	require.False(t, HasDocConvertableExtension("video.mp4", "video/mp4"))

	settings.Config.Integrations.Media.AlwaysUseOnlyOffice = true
	require.False(t, HasDocConvertableExtension("doc.pdf", "application/pdf"))
}

func TestDocPreviewExtensionsForSPA_includesFolioSet(t *testing.T) {
	exts := DocPreviewExtensionsForSPA()
	for folioExt := range FolioConvertable {
		name := folioExt[1:]
		require.True(t, slices.Contains(exts, name), "missing folio ext %q", name)
	}
}
