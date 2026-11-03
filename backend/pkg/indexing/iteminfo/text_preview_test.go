package iteminfo

import (
	"slices"
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
	require.True(t, IsTextPreviewable("Notes.TXT"))
	require.True(t, IsTextPreviewable("dir/readme.md"))
	require.False(t, IsTextPreviewable("image.svg"))
	require.False(t, IsTextPreviewable("noext"))
}

func TestTextPreviewExtensionsForSPA(t *testing.T) {
	groups := TextPreviewExtensionsForSPA()
	require.Contains(t, groups["markdown"], "md")
	require.Contains(t, groups["rtf"], "rtf")
	require.True(t, slices.Contains(groups["code"], "go"))
	require.False(t, slices.Contains(groups["code"], "svg"))
	for _, exts := range groups {
		for _, ext := range exts {
			require.NotContains(t, ext, ".", "extensions must not have a leading dot")
		}
	}
}
