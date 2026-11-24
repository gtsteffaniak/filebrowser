// folio.go is the pure-Go document preview fast path using gen2brain/folio.
// When folio cannot handle a file, callers fall back to the office converter (when configured).
package preview

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"path/filepath"
	"sync"

	"github.com/gen2brain/folio/doc"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/kovidgoyal/imaging"
)

const (
	// docSupersample renders a page at this multiple of the target box before the
	// final downscale. Rasterized text needs the extra pixels to stay crisp.
	docSupersample = 2
	// docOriginalDPI is the resolution used for "original" page previews.
	docOriginalDPI = 200
	// docMaxRenderEdge caps the longest edge of a rendered page, in pixels.
	docMaxRenderEdge = 4096
	// docMinDPI and docMaxDPI bound the computed render resolution.
	docMinDPI = 36.0
	docMaxDPI = 600.0
)

var renderMu sync.Mutex

// docRenderOptions controls the size and quality of one rendered document page.
type docRenderOptions struct {
	MaxWidth    int
	MaxHeight   int
	Fill        bool
	JpegQuality int
	Original    bool
}

// folioEnabled reports whether the folio fast path may run.
func folioEnabled() bool {
	return !settings.AlwaysUseOnlyOfficeEnabled()
}

func logFolioFallback(op, path string, err error) {
	if errors.Is(err, doc.ErrUnsupported) {
		logger.Warningf("folio does not support %s for %s (%v); using office converter if configured", op, path, err)
		return
	}
	logger.Errorf("folio %s failed for %s: %v; falling back to office converter if configured", op, path, err)
}

// renderDocPageJPEG renders one document page to JPEG. ok is true when folio returned bytes.
// name and mimetype should match the file being previewed; they control eligibility for
// folio (content sniffing does not depend on the path extension).
func renderDocPageJPEG(ctx context.Context, path string, pageNumber int, opts docRenderOptions, name, mimetype string) (data []byte, ok bool, err error) {
	if !folioEnabled() {
		return nil, false, nil
	}
	if name == "" {
		name = filepath.Base(path)
	}
	if !folioSupportsPreview(name, mimetype) {
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
		data, err := renderPageJPEG(ctx, path, name, mimetype, pageNumber, opts)
		done <- result{data: data, err: err}
	}()

	select {
	case <-ctx.Done():
		return nil, false, ctx.Err()
	case res := <-done:
		if res.err != nil {
			if !errors.Is(res.err, context.Canceled) && !errors.Is(res.err, context.DeadlineExceeded) {
				logFolioFallback("render", path, res.err)
			}
			return nil, false, res.err
		}
		if len(res.data) == 0 {
			return nil, false, fmt.Errorf("folio render returned empty jpeg")
		}
		return res.data, true, nil
	}
}

func renderPageJPEG(ctx context.Context, path, name, mimetype string, pageNumber int, opts docRenderOptions) ([]byte, error) {
	renderMu.Lock()
	defer renderMu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}

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

	boxed := !opts.Original && opts.MaxWidth > 0 && opts.MaxHeight > 0
	var dpi float64
	if boxed {
		dpi = docBoxRenderDPI(p, d.Kind(), opts.MaxWidth, opts.MaxHeight, opts.Fill)
	} else {
		dpi = docOriginalRenderDPI(p, d.Kind())
	}

	rendered, err := p.ImageDPI(dpi)
	if err != nil {
		return nil, err
	}
	if rendered == nil {
		return nil, fmt.Errorf("folio rendered no image for page %d", pageNumber)
	}

	var img image.Image = rendered
	if boxed {
		// The page was rendered at docSupersample times the target, so this
		// single high-quality downscale produces the anti-aliased final image.
		if opts.Fill {
			img = imaging.Fill(img, opts.MaxWidth, opts.MaxHeight, imaging.Center, imaging.Lanczos)
		} else {
			img = imaging.Fit(img, opts.MaxWidth, opts.MaxHeight, imaging.Lanczos)
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

// docUnitDPI is the resolution at which folio's ImageDPI renders page units one to one:
// 72 for PDF and book pages, 96 for drawings (SVG).
func docUnitDPI(kind doc.Kind) float64 {
	if kind == doc.KindSVG {
		return 96
	}
	return 72
}

// docMaxEdgeDPI is the highest resolution at which the longest page edge stays within docMaxRenderEdge.
func docMaxEdgeDPI(p doc.Page, kind doc.Kind) float64 {
	b := p.Bounds()
	longest := float64(max(b.X1-b.X0, b.Y1-b.Y0))
	if longest <= 0 {
		return docMinDPI
	}
	return docUnitDPI(kind) * docMaxRenderEdge / longest
}

// docBoxRenderDPI picks the resolution for a page that will be scaled into a boxW x boxH target.
// Fill covers the box; fit stays inside it. The page is rendered at docSupersample times the target.
func docBoxRenderDPI(p doc.Page, kind doc.Kind, boxW, boxH int, fill bool) float64 {
	b := p.Bounds()
	pw, ph := float64(b.X1-b.X0), float64(b.Y1-b.Y0)
	if pw <= 0 || ph <= 0 {
		return docMinDPI
	}
	// Scale (pixels per page unit) needed for the supersampled render to reach the box on each axis.
	sx := float64(boxW*docSupersample) / pw
	sy := float64(boxH*docSupersample) / ph
	s := min(sx, sy)
	if fill {
		s = max(sx, sy)
	}
	dpi := min(s*docUnitDPI(kind), docMaxEdgeDPI(p, kind))
	return min(max(dpi, docMinDPI), docMaxDPI)
}

// docOriginalRenderDPI picks the resolution for "original" previews: docOriginalDPI, capped by docMaxRenderEdge.
func docOriginalRenderDPI(p doc.Page, kind doc.Kind) float64 {
	return min(docOriginalDPI, docMaxEdgeDPI(p, kind))
}
