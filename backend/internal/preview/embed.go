package preview

import (
	"bytes"
	"image"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func hasEmbeddedPreview(fileType, fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	if iteminfo.IsRawImage(ext) {
		return true
	}
	return strings.HasPrefix(fileType, "image/hei")
}

func isJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}

// maxEmbeddedPreviewPixels bounds the embedded JPEG that is decoded for orientation and resizing, so a
// crafted or corrupt file can't make the decode allocate memory proportional to a huge declared size.
// Camera previews are at most full sensor resolution (a 100MP medium format JPEG is about 102M pixels).
const maxEmbeddedPreviewPixels = 150_000_000

// embeddedPreviewWithinLimits reports whether a JPEG's declared dimensions are small enough to decode.
// Only the header is read. A JPEG whose header can't be parsed is let through, as before: the later
// steps handle it or fail on it the same way.
func embeddedPreviewWithinLimits(data []byte) bool {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return true
	}
	return int64(cfg.Width)*int64(cfg.Height) <= maxEmbeddedPreviewPixels
}
