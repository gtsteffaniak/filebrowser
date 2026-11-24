package preview

import (
	// Decoders for embedded images; the resize path needs them registered.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"

	"github.com/gtsteffaniak/filebrowser/backend/internal/embeddedpreview"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// useEmbeddedPreview reports whether the embedded image may be used for this file.
// With "Always use office for document previews" on, office files keep the office converter.
func useEmbeddedPreview(file iteminfo.ExtendedFileInfo) bool {
	if file.OnlyOfficeId != "" && settings.AlwaysUseOnlyOfficeEnabled() {
		return false
	}
	return true
}

// embeddedImageDecodes checks that embedded bytes are a usable image: the header is within the
// pixel budget and the whole image decodes. A corrupt, stale, or oversized thumbnail falls through
// instead of failing or exhausting memory in the resize step.
func embeddedImageDecodes(data []byte) bool {
	return embeddedpreview.Usable(data)
}
