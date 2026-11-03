package web

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
)

func TestErrToStatusUnsupportedPreviewFormat(t *testing.T) {
	status := ErrToStatus(preview.ErrUnsupportedFormat)
	if status != http.StatusUnsupportedMediaType {
		t.Fatalf("ErrToStatus() = %d, want %d", status, http.StatusUnsupportedMediaType)
	}
}

func TestErrToStatusWrapsUnsupportedPreviewFormat(t *testing.T) {
	err := errors.Join(errors.New("preview failed"), preview.ErrUnsupportedFormat)
	status := ErrToStatus(err)
	if status != http.StatusUnsupportedMediaType {
		t.Fatalf("ErrToStatus() = %d, want %d", status, http.StatusUnsupportedMediaType)
	}
}

func TestIsViewerOriginalPreview(t *testing.T) {
	tests := []struct {
		name     string
		size     string
		file     string
		fileType string
		docs     bool // the server can render documents to an image
		want     bool
	}{
		{"Fuji RAW viewer", "original", "DSCF0308.RAF", "image/x-fuji-raf", false, true},
		{"Canon RAW viewer", "original", "IMG_0001.CR3", "image/x-canon-cr3", false, true},
		{"Adobe DNG viewer", "original", "a.dng", "image/x-adobe-dng", false, true},
		{"HEIC viewer", "original", "a.heic", "image/heic", false, true},
		{"HEIF viewer", "original", "a.heif", "image/heif", false, true},
		{"rendered document viewer", "original", "a.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true, true},
		{"documents need the server to be able to render them", "original", "a.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", false, false},
		{"RAW thumbnail stays disabled", "small", "DSCF0308.RAF", "image/x-fuji-raf", true, false},
		{"RAW large stays disabled", "large", "DSCF0308.RAF", "image/x-fuji-raf", true, false},
		{"no size stays disabled", "", "DSCF0308.RAF", "image/x-fuji-raf", true, false},
		{"video poster stays disabled", "original", "a.mov", "video/quicktime", true, false},
		{"audio cover stays disabled", "original", "a.mp3", "audio/mpeg", true, false},
		{"JPEG is opened directly, not through previews", "original", "a.jpg", "image/jpeg", true, false},
		{"PDF is opened directly, not through previews", "original", "a.pdf", "application/pdf", true, false},
		{"text stays disabled", "original", "a.txt", "text/plain", true, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isViewerOriginalPreview(tt.size, tt.file, tt.fileType, tt.docs); got != tt.want {
				t.Fatalf("isViewerOriginalPreview(%q, %q, %q, %v) = %v, want %v", tt.size, tt.file, tt.fileType, tt.docs, got, tt.want)
			}
		})
	}
}
