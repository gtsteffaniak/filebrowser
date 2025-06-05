//go:build !mupdf
// +build !mupdf

package preview

import "github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"

func docEnabled() bool {
	// This function checks if the PDF support is enabled.
	// In a real implementation, this might check a build tag or configuration.
	return false
}

func (s *Service) GenerateImageFromDoc(file iteminfo.ExtendedFileInfo, tempFilePath string, pageNumber int) ([]byte, error) { // 1. Serialize access to the entire go-fitz operation block
	s.docGenMutex.Lock()
	defer s.docGenMutex.Unlock()

	return nil, nil
}
