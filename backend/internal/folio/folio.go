// Package folio is the pure-Go document preview fast path using gen2brain/folio.
// When folio cannot handle a file, callers fall back to the office converter (when configured).
package folio

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gen2brain/folio/doc"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/kovidgoyal/imaging"
)

var renderMu sync.Mutex

// RenderOptions controls thumbnail sizing for one rendered page.
type RenderOptions struct {
	MaxWidth    int
	MaxHeight   int
	Fill        bool
	JpegQuality int
	Original    bool
}

// Enabled reports whether the folio fast path may run.
func Enabled() bool {
	return !settings.AlwaysUseOnlyOfficeEnabled()
}

// SupportsExtension reports whether folio targets the file extension (including leading dot).
func SupportsExtension(ext string) bool {
	ext = strings.ToLower(ext)
	if !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return iteminfo.FolioConvertable[ext]
}

func logFallback(op, path string, err error) {
	if errors.Is(err, doc.ErrUnsupported) {
		logger.Warningf("folio does not support %s for %s (%v); using office converter if configured", op, path, err)
		return
	}
	logger.Errorf("folio %s failed for %s: %v; falling back to office converter if configured", op, path, err)
}

// RenderPageJPEG renders one document page to JPEG. ok is true when folio returned bytes.
// name and mimetype should match the file being previewed; they control eligibility for
// folio (content sniffing does not depend on the path extension).
func RenderPageJPEG(ctx context.Context, path string, pageNumber int, opts RenderOptions, name, mimetype string) (data []byte, ok bool, err error) {
	if !Enabled() {
		return nil, false, nil
	}
	if name == "" {
		name = filepath.Base(path)
	}
	if !SupportsPreview(name, mimetype) {
		return nil, false, nil
	}
	if ctx.Err() != nil {
		return nil, false, ctx.Err()
	}

	type result struct {
		data []byte
		err  error
	}
	done := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- result{err: fmt.Errorf("folio render panicked: %v", r)}
			}
		}()
		data, err := renderPageJPEG(path, name, mimetype, pageNumber, opts)
		done <- result{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case res := <-done:
		if res.err != nil {
			logFallback("render", path, res.err)
			return nil, false, res.err
		}
		if len(res.data) == 0 {
			return nil, false, fmt.Errorf("folio render returned empty jpeg")
		}
		return res.data, true, nil
	}
}

func renderPageJPEG(path, name, mimetype string, pageNumber int, opts RenderOptions) ([]byte, error) {
	renderMu.Lock()
	defer renderMu.Unlock()

	d, err := openDocument(path, name, mimetype)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	if pageNumber < 0 || pageNumber >= d.NumPages() {
		return nil, fmt.Errorf("invalid page number %d (pages: %d)", pageNumber, d.NumPages())
	}

	p, err := d.Page(pageNumber)
	if err != nil {
		return nil, err
	}

	var img image.Image
	if opts.Original || opts.MaxWidth <= 0 || opts.MaxHeight <= 0 {
		const maxEdge = 4096
		rgba, err := p.ImageDPI(150)
		if err != nil {
			return nil, err
		}
		img = rgba
		w, h := rgba.Bounds().Dx(), rgba.Bounds().Dy()
		if w > maxEdge || h > maxEdge {
			scale := min(float64(maxEdge)/float64(w), float64(maxEdge)/float64(h))
			newW := max(1, int(float64(w)*scale))
			newH := max(1, int(float64(h)*scale))
			img = imaging.Resize(img, newW, newH, imaging.Lanczos)
		}
	} else {
		rgba, err := p.ImageDPI(150)
		if err != nil {
			return nil, err
		}
		img = rgba
		if opts.Fill {
			img = imaging.Fill(img, opts.MaxWidth, opts.MaxHeight, imaging.Center, imaging.Lanczos)
		} else {
			b := img.Bounds()
			if b.Dx() > opts.MaxWidth || b.Dy() > opts.MaxHeight {
				img = imaging.Fit(img, opts.MaxWidth, opts.MaxHeight, imaging.Lanczos)
			}
		}
	}

	q := opts.JpegQuality
	if q <= 0 {
		q = 85
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: q}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
