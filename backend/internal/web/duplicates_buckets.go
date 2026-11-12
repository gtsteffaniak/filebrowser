package web

import (
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// typeBucketKey returns the MIME/extension bucket used inside a fixed-size group.
// When the MIME type already implies a single on-disk format, extension is omitted
// as a redundant pre-filter. Otherwise a normalized lowercase extension is appended.
//
// JPEG aliases: ".jpg", ".jpeg", and ".jpe" normalize to "jpg" so differently named
// JPEGs stay in one bucket without splitting the MIME-implied image/jpeg group.
func typeBucketKey(file *iteminfo.FileInfo) string {
	mime := file.Type
	if mimeImpliesExtension(mime) {
		return mime
	}
	return mime + "\x1f" + normalizedFileExtension(file.Name)
}

func mimeImpliesExtension(mime string) bool {
	switch mime {
	case "image/jpeg", "image/png", "image/gif", "image/webp", "image/bmp", "image/tiff",
		"video/mp4", "video/webm", "video/quicktime", "video/x-msvideo",
		"audio/mpeg", "audio/wav", "audio/flac", "audio/ogg",
		"application/pdf", "application/zip", "application/gzip",
		"text/plain", "text/html", "text/css", "application/json":
		return true
	default:
		return false
	}
}

// normalizedFileExtension returns a lowercase extension without the dot, folding JPEG aliases to "jpg".
func normalizedFileExtension(name string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
	switch ext {
	case "jpeg", "jpe":
		return "jpg"
	default:
		return ext
	}
}

func groupFilesByTypeBucket(files []*iteminfo.FileInfo) map[string][]*iteminfo.FileInfo {
	grouped := make(map[string][]*iteminfo.FileInfo)
	for _, file := range files {
		key := typeBucketKey(file)
		grouped[key] = append(grouped[key], file)
	}
	return grouped
}
