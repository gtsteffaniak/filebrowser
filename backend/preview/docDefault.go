//go:build !mupdf

package preview

import (
	"context"

	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
)

func docEnabled() bool {
	// This function checks if the PDF support is enabled.
	// In a real implementation, this might check a build tag or configuration.
	return false
}

func (s *Service) GenerateImageFromDoc(ctx context.Context, file iteminfo.ExtendedFileInfo, tempFilePath string, pageNumber int) ([]byte, error) {
	// Reference it to prevent unused field warning when building without mupdf
	_ = &s.docGenMutex
	return nil, nil
}
