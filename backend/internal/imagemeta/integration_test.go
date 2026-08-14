//go:build !386 && !arm

package imagemeta

import (
	"os"
	"testing"
)

func TestHeicTransformOrientationMapping(t *testing.T) {
	tests := []struct {
		name string
		t    heicTransform
		want string
	}{
		{
			name: "irot normal",
			t:    heicTransform{irot: 0, found: true},
			want: "Horizontal (normal)",
		},
		{
			name: "irot 90 cw",
			t:    heicTransform{irot: 3, found: true},
			want: "Rotate 90 CW",
		},
		{
			name: "missing",
			t:    heicTransform{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := heicTransformOrientation(tt.t); got != tt.want {
				t.Fatalf("heicTransformOrientation() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectTIFFPreviewCandidatesNEF(t *testing.T) {
	const path = "/Users/steffag/Downloads/raw images/_Z721767.NEF"
	if _, err := os.Stat(path); err != nil {
		t.Skip("sample NEF not available:", path)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	candidates, err := collectTIFFPreviewCandidates(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := readLargestJPEGPreview(f, candidates)
	if err != nil {
		t.Fatal(err)
	}
	if !IsJPEG(data) {
		t.Fatalf("expected JPEG preview, got %d bytes", len(data))
	}
	if len(data) < 1_000_000 {
		t.Fatalf("preview too small: %d bytes", len(data))
	}
}

func TestGetOrientationHEICConflict(t *testing.T) {
	const path = "/Users/steffag/Downloads/heic/IMG_2919.HEIC"
	if _, err := os.Stat(path); err != nil {
		t.Skip("sample HEIC not available:", path)
	}
	got := GetOrientation(t.Context(), path)
	if got != "Horizontal (normal)" {
		t.Fatalf("GetOrientation() = %q, want Horizontal (normal)", got)
	}
}

func TestGetOrientationHEICRotate(t *testing.T) {
	const path = "/Users/steffag/Downloads/heic/IMG_6660.heic"
	if _, err := os.Stat(path); err != nil {
		t.Skip("sample HEIC not available:", path)
	}
	got := GetOrientation(t.Context(), path)
	if got != "Rotate 90 CW" {
		t.Fatalf("GetOrientation() = %q, want Rotate 90 CW", got)
	}
}

func TestExtractEmbeddedPreviewNEF(t *testing.T) {
	paths := []string{
		"/Users/steffag/Downloads/raw images/_Z721767.NEF",
		"/Users/steffag/Downloads/raw images/_DSC5331.NEF",
		"/Users/steffag/Downloads/raw images/_DSC5331 (2).NEF",
	}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			t.Skip("sample NEF not available:", path)
		}
		data, err := ExtractEmbeddedPreview(t.Context(), path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if !IsJPEG(data) {
			t.Fatalf("%s: expected JPEG preview, got %d bytes", path, len(data))
		}
		if len(data) < 1_000_000 {
			t.Fatalf("%s: preview too small (%d bytes), expected full JpgFromRaw", path, len(data))
		}
	}
}
