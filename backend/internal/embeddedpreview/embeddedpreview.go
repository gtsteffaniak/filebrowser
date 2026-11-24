// Package embeddedpreview returns a preview image that is already stored inside a file:
// camera RAW and HEIC previews, and thumbnails or merged images in zip-based documents and
// project files. Every extractor returns ErrNotFound when a file has no usable image, so callers
// fall back to go-media, ffmpeg, and the document pipeline. Nothing is extracted to disk.
package embeddedpreview

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/imagemeta"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

// ErrNotFound means the file has no embedded image this package can read.
var ErrNotFound = errors.New("no embedded preview image found")

// Result is the embedded image and how the caller should treat it.
type Result struct {
	Data []byte
	// Ext is the lowercase extension of the embedded image, for example ".png".
	Ext string
	// Oriented is true when EXIF orientation applies to Data (camera RAW and HEIC).
	Oriented bool
}

// Extractor reads one family of formats.
type Extractor interface {
	Supports(ext, mimetype string) bool
	Extract(ctx context.Context, path string) (Result, error)
}

var extractors = []Extractor{
	rawExtractor{},
	zipExtractor{exts: []string{".kra"}, pick: pickKrita},
	zipExtractor{exts: []string{".ora"}, pick: pickORA},
	zipExtractor{exts: []string{".sketch"}, pick: pickSketch},
	zipExtractor{exts: []string{".mscz"}, pick: pickMuseScore},
	zipExtractor{exts: []string{".odt", ".ods", ".odp", ".odg"}, pick: pickODF},
	zipExtractor{exts: []string{".docx", ".xlsx", ".pptx"}, pick: pickOOXML},
	zipExtractor{exts: []string{".cbz"}, pick: pickCBZ},
	zipExtractor{exts: []string{".xps"}, pick: pickXPS},
}

func lookup(name, mimetype string) (Extractor, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	for _, e := range extractors {
		if e.Supports(ext, mimetype) {
			return e, true
		}
	}
	return nil, false
}

// Supports reports whether an extractor in this package handles the file.
func Supports(name, mimetype string) bool {
	_, ok := lookup(name, mimetype)
	return ok
}

// Extract returns the embedded image for the file at path. It returns ErrNotFound when the
// format has no extractor or the file has no usable image.
func Extract(ctx context.Context, name, mimetype, path string) (Result, error) {
	e, ok := lookup(name, mimetype)
	if !ok {
		return Result{}, ErrNotFound
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	return e.Extract(ctx, path)
}

// rawExtractor wraps imagemeta for camera RAW and HEIC files.
type rawExtractor struct{}

func (rawExtractor) Supports(ext, mimetype string) bool {
	return iteminfo.IsRawImage(ext) || strings.HasPrefix(mimetype, "image/hei")
}

func (rawExtractor) Extract(ctx context.Context, path string) (Result, error) {
	data, err := imagemeta.ExtractEmbeddedPreview(ctx, path)
	if err != nil {
		return Result{}, err
	}
	if len(data) == 0 {
		return Result{}, ErrNotFound
	}
	return Result{Data: data, Ext: ".jpg", Oriented: true}, nil
}

// zipExtractor handles a zip container, using pick to choose the entry.
type zipExtractor struct {
	exts []string
	pick func(zr *zip.Reader) (Result, error)
}

func (z zipExtractor) Supports(ext, _ string) bool {
	for _, e := range z.exts {
		if e == ext {
			return true
		}
	}
	return false
}

func (z zipExtractor) Extract(_ context.Context, path string) (Result, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return Result{}, fmt.Errorf("open archive: %w", err)
	}
	defer zr.Close()
	return z.pick(&zr.Reader)
}
