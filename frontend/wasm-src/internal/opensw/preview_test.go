package opensw

import (
	"testing"
)

func TestExtractPreviewInvalid(t *testing.T) {
	// Empty data should return error
	_, err := ExtractSolidWorksPreviewBytes([]byte{})
	if err == nil {
		t.Fatalf("expected error for empty data")
	}

	// Random garbage should return error
	_, err = ExtractSolidWorksPreviewBytes([]byte("random non-cad content"))
	if err == nil {
		t.Fatalf("expected error for non-cad data")
	}
}

func TestDIBToPNGInvalid(t *testing.T) {
	// Empty or truncated DIB data
	_, err := dibToPNG([]byte{})
	if err == nil {
		t.Fatalf("expected error for empty DIB")
	}

	// Header too small
	_, err = dibToPNG(make([]byte, 20))
	if err == nil {
		t.Fatalf("expected error for header too small")
	}
}

func TestOpenContainerEmpty(t *testing.T) {
	_, err := OpenContainer([]byte{})
	if err == nil {
		t.Fatalf("expected error for empty container")
	}
}
