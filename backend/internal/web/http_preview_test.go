package web

import (
	"errors"
	"net/http"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/preview"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
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
	prev := settings.Env.MuPdfAvailable
	defer func() { settings.Env.MuPdfAvailable = prev }()
	settings.Env.MuPdfAvailable = true

	tests := []struct {
		name     string
		size     string
		file     string
		fileType string
		want     bool
	}{
		{"Fuji RAW viewer", "original", "DSCF0308.RAF", "image/x-fuji-raf", true},
		{"Canon RAW viewer", "original", "IMG_0001.CR3", "image/x-canon-cr3", true},
		{"Adobe DNG viewer", "original", "a.dng", "image/x-adobe-dng", true},
		{"HEIC viewer", "original", "a.heic", "image/heic", true},
		{"HEIF viewer", "original", "a.heif", "image/heif", true},
		{"rendered document viewer", "original", "a.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document", true},
		{"RAW thumbnail stays disabled", "small", "DSCF0308.RAF", "image/x-fuji-raf", false},
		{"RAW large stays disabled", "large", "DSCF0308.RAF", "image/x-fuji-raf", false},
		{"no size stays disabled", "", "DSCF0308.RAF", "image/x-fuji-raf", false},
		{"video poster stays disabled", "original", "a.mov", "video/quicktime", false},
		{"audio cover stays disabled", "original", "a.mp3", "audio/mpeg", false},
		{"JPEG is opened directly, not through previews", "original", "a.jpg", "image/jpeg", false},
		{"PDF is opened directly, not through previews", "original", "a.pdf", "application/pdf", false},
		{"text stays disabled", "original", "a.txt", "text/plain", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isViewerOriginalPreview(tt.size, tt.file, tt.fileType); got != tt.want {
				t.Fatalf("isViewerOriginalPreview(%q, %q, %q) = %v, want %v", tt.size, tt.file, tt.fileType, got, tt.want)
			}
		})
	}
}

func TestIsViewerOriginalPreviewDocumentsNeedMuPdf(t *testing.T) {
	prev := settings.Env.MuPdfAvailable
	defer func() { settings.Env.MuPdfAvailable = prev }()
	settings.Env.MuPdfAvailable = false
	if isViewerOriginalPreview("original", "a.docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document") {
		t.Fatal("documents can't be rendered without MuPDF")
	}
	// RAW and HEIC don't depend on MuPDF.
	if !isViewerOriginalPreview("original", "a.raf", "image/x-fuji-raf") {
		t.Fatal("RAW should still be served without MuPDF")
	}
}
