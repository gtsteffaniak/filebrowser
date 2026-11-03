package preview

import (
	"context"
	"fmt"
	"io"
	"os"
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

	docPath := file.RealPath
	if strings.HasPrefix(file.Type, "text") && !strings.HasSuffix(file.RealPath, ".txt") {
		originalFile, err := os.Open(file.RealPath)
		if err != nil {
			logger.Errorf("text snippet preview: failed to open file '%s': %v", file.Name, err)
			return nil, fmt.Errorf("text snippet: failed to open file")
		}
		defer originalFile.Close()

		buffer := make([]byte, 1024)
		n, readErr := originalFile.Read(buffer)
		if readErr != nil && readErr != io.EOF {
			logger.Errorf("text snippet preview: failed to read file '%s': %v", file.Name, readErr)
			return nil, fmt.Errorf("text snippet: failed to read file")
		}
		if n == 0 {
			return nil, fmt.Errorf("text snippet: file is empty or unreadable")
		}

		tempFile, err := os.Create(tempFilePath)
		if err != nil {
			logger.Errorf("text snippet preview: failed to create temp file for '%s': %v", file.Name, err)
			return nil, fmt.Errorf("text snippet: failed to create temporary file")
		}
		defer os.Remove(tempFilePath)
		if _, err := tempFile.Write(buffer[:n]); err != nil {
			tempFile.Close()
			os.Remove(tempFilePath)
			logger.Errorf("text snippet preview: failed to write temp file for '%s': %v", file.Name, err)
			return nil, fmt.Errorf("text snippet: failed to write temporary file")
		}
		if err := tempFile.Close(); err != nil {
			os.Remove(tempFilePath)
			logger.Errorf("text snippet preview: failed to close temp file for '%s': %v", file.Name, err)
			return nil, fmt.Errorf("text snippet: failed to close temporary file")
		}
		docPath = tempFilePath
	}

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
