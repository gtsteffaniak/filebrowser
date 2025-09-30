package settings

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestVideoPreviewDefaults(t *testing.T) {
	// Test 1: No videoPreview config at all - should enable all
	t.Run("NoVideoPreviewConfig", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  media:
    convert:
      imagePreview:
        heic: false
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// All video formats should be enabled by default
			for _, format := range AllVideoPreviewTypes {
				if !config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be enabled by default, but it was disabled", format)
				}
			}
		})
	})

	// Test 2: Empty videoPreview config - should enable all
	t.Run("EmptyVideoPreviewConfig", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  media:
    convert:
      imagePreview:
        heic: false
      videoPreview: {}
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// All video formats should be enabled by default
			for _, format := range AllVideoPreviewTypes {
				if !config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be enabled by default, but it was disabled", format)
				}
			}
		})
	})

	// Test 3: Partial videoPreview config - should enable all except explicitly disabled
	t.Run("PartialVideoPreviewConfig", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  media:
    convert:
      imagePreview:
        heic: false
      videoPreview:
        mp4: true
        webm: false
        mkv: false
        wmv: false
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// Check explicitly set formats
			if !config.Integrations.Media.Convert.VideoPreview[MP4VideoPreview] {
				t.Error("Expected mp4 to be enabled")
			}
			if config.Integrations.Media.Convert.VideoPreview[WebMVideoPreview] {
				t.Error("Expected webm to be disabled")
			}
			if config.Integrations.Media.Convert.VideoPreview[MKVVideoPreview] {
				t.Error("Expected mkv to be disabled")
			}
			if config.Integrations.Media.Convert.VideoPreview[WMVVideoPreview] {
				t.Error("Expected wmv to be disabled")
			}

			// Check that unmentioned formats are still enabled
			unmentionedFormats := []VideoPreviewType{
				AVIVideoPreview, MOVVideoPreview, FLVVideoPreview,
				M4VVideoPreview, ThreeGPVideoPreview, TSVideoPreview, VOBVideoPreview,
			}
			for _, format := range unmentionedFormats {
				if !config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be enabled (not mentioned in config), but it was disabled", format)
				}
			}
		})
	})

	// Test 4: All videoPreview config - should respect all settings
	t.Run("AllVideoPreviewConfig", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  media:
    convert:
      imagePreview:
        heic: false
      videoPreview:
        mp4: true
        webm: true
        mov: true
        avi: false
        mkv: true
        flv: false
        wmv: true
        m4v: false
        3gp: true
        3g2: false
        ts: true
        m2ts: false
        vob: true
        asf: false
        mpg: true
        mpeg: false
        f4v: true
        ogv: false
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// Check all explicitly set formats
			expectedEnabled := []VideoPreviewType{
				MP4VideoPreview, WebMVideoPreview, MOVVideoPreview, MKVVideoPreview,
				WMVVideoPreview, ThreeGPVideoPreview, TSVideoPreview, VOBVideoPreview,
				MPGVideoPreview, F4VVideoPreview,
			}
			expectedDisabled := []VideoPreviewType{
				AVIVideoPreview, FLVVideoPreview, M4VVideoPreview,
				ThreeGP2VideoPreview, M2TSVideoPreview, ASFVideoPreview,
				MPEGVideoPreview, OGVVideoPreview,
			}

			for _, format := range expectedEnabled {
				if !config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be enabled, but it was disabled", format)
				}
			}

			for _, format := range expectedDisabled {
				if config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be disabled, but it was enabled", format)
				}
			}
		})
	})

	// Test 5: No integrations section at all - should enable all
	t.Run("NoIntegrationsSection", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// All video formats should be enabled by default
			for _, format := range AllVideoPreviewTypes {
				if !config.Integrations.Media.Convert.VideoPreview[format] {
					t.Errorf("Expected %s to be enabled by default, but it was disabled", format)
				}
			}
		})
	})
}

func TestVideoPreviewFormatCategories(t *testing.T) {
	// Test that all video format categories are properly supported
	t.Run("FormatCategories", func(t *testing.T) {
		config := `
server:
  sources:
    - path: "."
integrations:
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`
		testVideoPreviewConfig(t, config, func(t *testing.T, config *Settings) {
			// Test format categories
			categories := map[string][]VideoPreviewType{
				"Common Web Formats":     {MP4VideoPreview, WebMVideoPreview, OGVVideoPreview},
				"Apple Formats":          {MOVVideoPreview, M4VVideoPreview},
				"Microsoft Formats":      {AVIVideoPreview, WMVVideoPreview, ASFVideoPreview},
				"Open Source":            {MKVVideoPreview, FLVVideoPreview},
				"Mobile Formats":         {ThreeGPVideoPreview, ThreeGP2VideoPreview},
				"Broadcast/Professional": {TSVideoPreview, M2TSVideoPreview},
				"Legacy Formats":         {VOBVideoPreview, MPGVideoPreview, MPEGVideoPreview, F4VVideoPreview},
			}

			for category, formats := range categories {
				t.Run(category, func(t *testing.T) {
					for _, format := range formats {
						if !config.Integrations.Media.Convert.VideoPreview[format] {
							t.Errorf("Expected %s (%s) to be enabled by default, but it was disabled", format, category)
						}
					}
				})
			}
		})
	})
}

func TestVideoPreviewFormatCount(t *testing.T) {
	// Test that we have the expected number of video formats
	t.Run("FormatCount", func(t *testing.T) {
		expectedCount := 18
		actualCount := len(AllVideoPreviewTypes)
		if actualCount != expectedCount {
			t.Errorf("Expected %d video formats, but got %d", expectedCount, actualCount)
		}

		// Test that all formats are unique
		formatMap := make(map[VideoPreviewType]bool)
		for _, format := range AllVideoPreviewTypes {
			if formatMap[format] {
				t.Errorf("Duplicate video format found: %s", format)
			}
			formatMap[format] = true
		}
	})
}

func BenchmarkVideoPreviewSetup(b *testing.B) {
	config := `
server:
  sources:
    - path: "."
integrations:
  media:
    convert:
      videoPreview:
        mp4: true
        webm: false
        mkv: true
        wmv: false
  office:
    url: "http://localhost:8080"
    secret: "test-secret"
`

	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "benchmark_video_preview_*.yaml")
	if err != nil {
		b.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	// Create a temporary directory for cache
	tmpCacheDir, err := os.MkdirTemp("", "benchmark_cache_*")
	if err != nil {
		b.Fatalf("Error creating temp cache dir: %v", err)
	}
	defer os.RemoveAll(tmpCacheDir)

	// Modify config to use absolute temp directory
	modifiedConfig := strings.Replace(config, "server:", fmt.Sprintf("server:\n  cacheDir: %s", tmpCacheDir), 1)

	_, err = tmpFile.WriteString(modifiedConfig)
	if err != nil {
		b.Fatalf("Error writing config: %v", err)
	}
	tmpFile.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Initialize(tmpFile.Name())
	}
}

// Helper function to test video preview configuration
func testVideoPreviewConfig(t *testing.T, configContent string, testFunc func(t *testing.T, config *Settings)) {
	// Create temporary config file
	tmpFile, err := os.CreateTemp("", "test_video_preview_*.yaml")
	if err != nil {
		t.Fatalf("Error creating temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(configContent)
	if err != nil {
		t.Fatalf("Error writing config: %v", err)
	}
	tmpFile.Close()

	// Initialize settings
	Initialize(tmpFile.Name())

	// Run the test function
	testFunc(t, &Config)
}
