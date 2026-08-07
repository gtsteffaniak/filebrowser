package preview

import (
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
