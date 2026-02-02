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
	// Serialize access (required for CGO thread safety, even though go-fitz is not available)
	s.docGenMutex.Lock()
	defer s.docGenMutex.Unlock()

	return nil, nil
}
