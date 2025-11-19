package ffmpeg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetOrientationFilter(t *testing.T) {
	service := &FFmpegService{}

	tests := []struct {
		name        string
		orientation string
		expected    string
	}{
		{
			name:        "Normal orientation",
			orientation: "Horizontal (normal)",
			expected:    "",
		},
		{
			name:        "Normal orientation alternative",
			orientation: "Top-left",
			expected:    "",
		},
		{
			name:        "Rotate 90 CW",
			orientation: "Rotate 90 CW",
			expected:    ",transpose=1",
		},
		{
			name:        "Rotate 90 CW alternative",
			orientation: "Right-top",
			expected:    ",transpose=1",
		},
		{
			name:        "Rotate 180",
			orientation: "Rotate 180",
			expected:    ",transpose=1,transpose=1",
		},
		{
			name:        "Rotate 180 alternative",
			orientation: "Bottom-right",
			expected:    ",transpose=1,transpose=1",
		},
		{
			name:        "Rotate 270 CW",
			orientation: "Rotate 270 CW",
			expected:    ",transpose=2",
		},
		{
			name:        "Rotate 270 CW alternative",
			orientation: "Left-bottom",
			expected:    ",transpose=2",
		},
		{
			name:        "Mirror horizontal",
			orientation: "Mirror horizontal",
			expected:    ",hflip",
		},
		{
			name:        "Mirror horizontal alternative",
			orientation: "Top-right",
			expected:    ",hflip",
		},
		{
			name:        "Mirror vertical",
			orientation: "Mirror vertical",
			expected:    ",vflip",
		},
		{
			name:        "Mirror vertical alternative",
			orientation: "Bottom-left",
			expected:    ",vflip",
		},
		{
			name:        "Mirror horizontal and rotate 270 CW",
			orientation: "Mirror horizontal and rotate 270 CW",
			expected:    ",transpose=0",
		},
		{
			name:        "Mirror horizontal and rotate 270 CW alternative",
			orientation: "Right-bottom",
			expected:    ",transpose=0",
		},
		{
			name:        "Mirror horizontal and rotate 90 CW",
			orientation: "Mirror horizontal and rotate 90 CW",
			expected:    ",transpose=3",
		},
		{
			name:        "Mirror horizontal and rotate 90 CW alternative",
			orientation: "Left-top",
			expected:    ",transpose=3",
		},
		{
			name:        "Unknown orientation",
			orientation: "Unknown Orientation",
			expected:    "",
		},
		{
			name:        "Empty orientation",
			orientation: "",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.GetOrientationFilter(tt.orientation)
			if result != tt.expected {
				t.Errorf("GetOrientationFilter(%q) = %q, want %q", tt.orientation, result, tt.expected)
			}
		})
	}
}

func TestGetImageOrientation(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "ffmpeg_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	service := NewFFmpegService(2, false, "/tmp/test_cache")

	t.Run("Non-existent file", func(t *testing.T) {
		nonExistentFile := filepath.Join(tempDir, "non_existent.heic")
		orientation, err := service.GetImageOrientation(nonExistentFile)

		// Should return default orientation when exiftool fails
		if err != nil {
			t.Errorf("Expected no error for non-existent file, got: %v", err)
		}
		if orientation != "Horizontal (normal)" {
			t.Errorf("Expected default orientation 'Horizontal (normal)', got: %q", orientation)
		}
	})

	t.Run("Empty file path", func(t *testing.T) {
		orientation, err := service.GetImageOrientation("")

		// Should return default orientation when exiftool fails
		if err != nil {
			t.Errorf("Expected no error for empty file path, got: %v", err)
		}
		if orientation != "Horizontal (normal)" {
			t.Errorf("Expected default orientation 'Horizontal (normal)', got: %q", orientation)
		}
	})
}

// Benchmark the orientation filter generation
func BenchmarkGetOrientationFilter(b *testing.B) {
	service := &FFmpegService{}
	orientations := []string{
		"Horizontal (normal)",
		"Rotate 90 CW",
		"Rotate 180",
		"Mirror vertical",
		"Unknown Orientation",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		orientation := orientations[i%len(orientations)]
		_ = service.GetOrientationFilter(orientation)
	}
}

// Test table for all EXIF orientation values
func TestGetOrientationFilter_AllEXIFValues(t *testing.T) {
	service := &FFmpegService{}

	// Test all possible EXIF orientation values according to EXIF spec
	exifTests := []struct {
		exifValue             string
		description           string
		expectsTransformation bool
	}{
		{"1", "Top-left (normal)", false},
		{"2", "Top-right (flip horizontal)", true},
		{"3", "Bottom-right (rotate 180)", true},
		{"4", "Bottom-left (flip vertical)", true},
		{"5", "Left-top (transpose)", true},
		{"6", "Right-top (rotate 90 CW)", true},
		{"7", "Right-bottom (transverse)", true},
		{"8", "Left-bottom (rotate 270 CW)", true},
	}

	for _, tt := range exifTests {
		t.Run(tt.description, func(t *testing.T) {
			filter := service.GetOrientationFilter(tt.exifValue)

			if tt.expectsTransformation && filter == "" {
				t.Errorf("Expected transformation for EXIF value %s (%s), but got empty filter", tt.exifValue, tt.description)
			}
			if !tt.expectsTransformation && filter != "" {
				t.Errorf("Expected no transformation for EXIF value %s (%s), but got filter: %s", tt.exifValue, tt.description, filter)
			}
		})
	}
}
