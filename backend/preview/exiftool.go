package preview

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

// hasEmbeddedPreview reports whether we should try exiftool for an embedded
// preview. Only true for types that often have one and lack a cheap native decode
func hasEmbeddedPreview(fileType string, fileName string) bool {
	ext := strings.ToLower(filepath.Ext(fileName))
	// Raw camera formats: we can't decode natively; exiftool is the main way to get a preview.
	if iteminfo.IsRawImage(ext) {
		return true
	}
	// HEIC/HEIF: often have embedded preview; we have FFmpeg fallback but exiftool can be faster.
	if strings.HasPrefix(fileType, "image/hei") {
		return true
	}
	return false
}

// ExtractEmbeddedPreview runs exiftool on the given file to extract an embedded
// preview image (e.g. JpgFromRaw, PreviewImage, ThumbnailImage). Returns the first
// non-empty result, or nil if exiftool is unavailable or no embedded preview exists.
// fileType is the MIME type (e.g. "image/x-canon-cr2", "image/heic", "video/quicktime").
func ExtractEmbeddedPreview(ctx context.Context, realPath, fileType string) ([]byte, error) {
	if realPath == "" {
		return nil, nil
	}
	path := settings.Config.Integrations.Media.ExiftoolPath
	if path == "" {
		return nil, nil
	}

	ext := strings.ToLower(filepath.Ext(realPath))

	// Tag order: raw often has JpgFromRaw/PreviewImage; HEIC/video use PreviewImage; JPEG has ThumbnailImage.
	tags := []string{"PreviewImage", "JpgFromRaw", "ThumbnailImage"}
	if iteminfo.IsRawImage(ext) {
		tags = []string{"JpgFromRaw", "PreviewImage", "ThumbnailImage"}
	}

	for _, tag := range tags {
		out, err := runExiftoolTag(ctx, path, realPath, tag)
		if err != nil {
			continue
		}
		if len(out) > 0 {
			return out, nil
		}
	}
	return nil, nil
}

// GetOrientation returns the EXIF orientation string for the file (e.g. "Rotate 90 CW", "Horizontal (normal)").
// Uses exiftool -Orientation -s3. Returns empty string if exiftool is unavailable or orientation cannot be read.
func GetOrientation(ctx context.Context, realPath string) string {
	if realPath == "" {
		return ""
	}
	path := settings.Config.Integrations.Media.ExiftoolPath
	if path == "" {
		return ""
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-Orientation", "-s3", realPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func runExiftoolTag(ctx context.Context, exiftoolPath, realPath, tag string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, exiftoolPath, "-b", "-"+tag, realPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return out, nil
}
