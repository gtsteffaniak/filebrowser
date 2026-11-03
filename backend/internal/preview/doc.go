package preview

import (
	"context"
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/folio"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func (s *Service) GenerateImageFromDoc(ctx context.Context, file iteminfo.ExtendedFileInfo, pageNumber int, previewSize string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	opts, err := folioRenderOptions(previewSize)
	if err != nil {
		return nil, err
	}

	imageBytes, ok, err := folio.RenderPageJPEG(timeoutCtx, file.RealPath, pageNumber, opts, file.Name, file.Type)
	if !ok {
		if err != nil {
			return nil, err
		}
		return nil, nil
	}
	if timeoutCtx.Err() != nil {
		return nil, fmt.Errorf("document preview generation timed out after 2 seconds for '%s'", file.Name)
	}
	return imageBytes, nil
}

func folioRenderOptions(previewSize string) (folio.RenderOptions, error) {
	if previewSize == "original" {
		return folio.RenderOptions{Original: true, JpegQuality: QualityHigh.jpegQuality()}, nil
	}
	opts, err := getPreviewOptions(previewSize)
	if err != nil {
		return folio.RenderOptions{}, err
	}
	return folio.RenderOptions{
		MaxWidth:    opts.Width,
		MaxHeight:   opts.Height,
		Fill:        opts.ResizeMode == ResizeModeFill,
		JpegQuality: opts.Quality.jpegQuality(),
	}, nil
}
