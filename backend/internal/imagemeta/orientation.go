package imagemeta

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	extimagemeta "github.com/evanoberholster/imagemeta"
	"github.com/evanoberholster/imagemeta/meta/exif"
	"github.com/evanoberholster/imagemeta/meta/exif/tag"
)

// GetOrientation returns the display orientation string for a file (e.g. "Rotate 90 CW",
// "Horizontal (normal)"). Returns empty string when orientation cannot be read.
func GetOrientation(ctx context.Context, path string) string {
	if ctx.Err() != nil || path == "" {
		return ""
	}

	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	ext := strings.ToLower(filepath.Ext(path))
	var ex exif.Exif
	switch ext {
	case ".heic", ".heif", ".heics":
		ex, err = extimagemeta.DecodeHeif(f)
	default:
		ex, err = extimagemeta.Decode(f)
	}
	if err != nil {
		return ""
	}
	if ctx.Err() != nil {
		return ""
	}

	if ex.IFD0.Orientation == 0 {
		return ""
	}
	return tag.ValueNameFor(tag.IFD0, tag.TagOrientation, uint32(ex.IFD0.Orientation))
}
