//go:build !pdf
// +build !pdf

package preview

func pdfEnabled() bool {
	// This function checks if the PDF support is enabled.
	// In a real implementation, this might check a build tag or configuration.
	return false
}

func (s *Service) GenerateImageFromPDF(pdfPath string, pageNumber int) ([]byte, error) {
	return nil, nil
}
