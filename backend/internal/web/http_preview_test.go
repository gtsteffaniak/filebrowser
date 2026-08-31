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
