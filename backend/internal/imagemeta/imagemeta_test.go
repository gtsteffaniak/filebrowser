//go:build !386 && !arm

package imagemeta

import (
	"os"
	"testing"

	"github.com/evanoberholster/imagemeta/meta/exif/tag"
)

func TestIsJPEG(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want bool
	}{
		{"empty", nil, false},
		{"jpeg soi", []byte{0xff, 0xd8, 0xff, 0xe0}, true},
		{"not jpeg", []byte{0x89, 0x50, 0x4e, 0x47}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsJPEG(tt.data); got != tt.want {
				t.Errorf("IsJPEG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRawImageExtension(t *testing.T) {
	if !isRawImageExtension(".cr2") {
		t.Fatal("expected .cr2 to be raw")
	}
	if isRawImageExtension(".jpg") {
		t.Fatal("expected .jpg not to be raw")
	}
}

func TestOrientationStringMapping(t *testing.T) {
	cases := map[uint32]string{
		1: "Horizontal (normal)",
		6: "Rotate 90 CW",
		8: "Rotate 270 CW",
	}
	for value, want := range cases {
		if got := tag.ValueNameFor(tag.IFD0, tag.TagOrientation, value); got != want {
			t.Errorf("orientation %d = %q, want %q", value, got, want)
		}
	}
}

func TestGetOrientationMissingFile(t *testing.T) {
	if got := GetOrientation(t.Context(), "/nonexistent/file.cr2"); got != "" {
		t.Fatalf("GetOrientation() = %q, want empty", got)
	}
}

func TestExtractEmbeddedPreviewMissingFile(t *testing.T) {
	data, err := ExtractEmbeddedPreview(t.Context(), "/nonexistent/file.cr2")
	if err != nil {
		t.Fatalf("ExtractEmbeddedPreview() err = %v", err)
	}
	if data != nil {
		t.Fatalf("ExtractEmbeddedPreview() = %v, want nil", data)
	}
}

func TestReadFileRangeRejectsOversizedLength(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "preview-*")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	if _, err = f.Write([]byte{0xff, 0xd8}); err != nil {
		t.Fatal(err)
	}

	data, err := readFileRange(f, 0, maxPreviewReadSize+1)
	if err != nil {
		t.Fatalf("readFileRange() err = %v, want nil", err)
	}
	if data != nil {
		t.Fatalf("readFileRange() = %v, want nil", data)
	}
}

func TestExtractEmbeddedPreviewSkipsJPEG(t *testing.T) {
	data, err := ExtractEmbeddedPreview(t.Context(), "/nonexistent/file.jpg")
	if err != nil {
		t.Fatalf("ExtractEmbeddedPreview() err = %v", err)
	}
	if data != nil {
		t.Fatalf("ExtractEmbeddedPreview() = %v, want nil", data)
	}
}
