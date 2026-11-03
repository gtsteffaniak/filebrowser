package preview

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/folio"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

func (s *Service) GenerateImageFromDoc(ctx context.Context, file iteminfo.ExtendedFileInfo, tempFilePath string, pageNumber int, previewSize string) ([]byte, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	isText := strings.HasPrefix(file.Type, "text")
	docPath, cleanup, err := folioTextPath(file.RealPath, tempFilePath, isText)
	if err != nil {
		logger.Errorf("text preview: failed to prepare file '%s': %v", file.Name, err)
		return nil, fmt.Errorf("text preview: failed to prepare file")
	}
	defer cleanup()

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	opts, err := folioRenderOptions(previewSize)
	if err != nil {
		return nil, err
	}

	imageBytes, ok, err := folio.RenderPageJPEG(timeoutCtx, docPath, pageNumber, opts)
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
