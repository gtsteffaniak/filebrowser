package preview

import (
	"path/filepath"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/gtsteffaniak/go-logger/logger"
)

// isHEICFile checks if the file is a HEIC file based on MIME type or extension
func isHEICFile(file iteminfo.ExtendedFileInfo) bool {
	// Check by MIME type first
	if file.Type == "image/heic" {
		return true
	}

	// Check by file extension (in case MIME type detection fails)
	ext := strings.ToLower(filepath.Ext(file.Name))
	return ext == ".heic" || ext == ".heif"
}

// processHEICFile handles HEIC file processing by delegating to FFmpeg conversion
func (s *Service) processHEICFile(file iteminfo.ExtendedFileInfo, previewSize string) ([]byte, error) {
	logger.Infof("🔄 HEIC: Processing HEIC file %s (size: %s)", filepath.Base(file.Name), previewSize)

	// Delegate to FFmpeg conversion logic
	return s.convertHEICToJPEGWithFFmpeg(file.RealPath, previewSize)
}
