package preview

import (
	"context"
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

const (
	// docPreviewTimeout bounds small, large and xlarge page renders.
	docPreviewTimeout = 2 * time.Second
	// docOriginalTimeout bounds "original" renders, which are higher resolution.
	docOriginalTimeout = 10 * time.Second
)

func (s *Service) GenerateImageFromDoc(ctx context.Context, file iteminfo.ExtendedFileInfo, pageNumber int, previewSize string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	timeout := docPreviewTimeout
	if previewSize == "original" {
		timeout = docOriginalTimeout
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	opts, err := folioRenderOptions(previewSize)
	if err != nil {
		return nil, err
	}

	imageBytes, ok, err := renderDocPageJPEG(timeoutCtx, file.RealPath, pageNumber, opts, file.Name, file.Type)
	if !ok {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	if timeoutCtx.Err() != nil {
		return nil, fmt.Errorf("document preview generation timed out after %s for '%s'", timeout, file.Name)
	}
	return imageBytes, nil
}

// folioRenderOptions maps a preview size onto the same dimensions, fill mode and JPEG quality
// the image pipeline uses (see getPreviewOptions); "original" renders at full resolution.
func folioRenderOptions(previewSize string) (docRenderOptions, error) {
	if previewSize == "original" {
		return docRenderOptions{Original: true, JpegQuality: QualityHigh.jpegQuality()}, nil
	}
	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return docRenderOptions{}, err
	}
	return docRenderOptions{
		MaxWidth:    opts.Width,
		MaxHeight:   opts.Height,
		Fill:        opts.ResizeMode == ResizeModeFill,
		JpegQuality: opts.Quality.jpegQuality(),
	}, nil
}
