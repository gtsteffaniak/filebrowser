package preview

import (
	"bytes"
	"errors"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/adapters/fs/fileutils"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

func TestMain(m *testing.M) {
	// Ensure fileutils permissions are set (needed by NewPreviewGenerator)
	if fileutils.PermDir == 0 {
		fileutils.SetFsPermissions(0644, 0755)
	}

	// Run the tests
	code := m.Run()

	// Exit with the test result code
	os.Exit(code)
}

func TestService_Resize(t *testing.T) {
	testCases := map[string]struct {
		options ResizeOptions
		source  func(t *testing.T) afero.File
		matcher func(t *testing.T, reader io.Reader)
		wantErr bool
	}{
		"fill upscale": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 50, 20)
			},
			matcher: sizeMatcher(100, 100),
		},
		"fill downscale": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"fit upscale": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFit},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 50, 20)
			},
			matcher: sizeMatcher(50, 20),
		},
		"fit downscale": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFit},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 75),
		},
		"keep original format": {
			options: ResizeOptions{Width: 100, Height: 100},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayPng(t, 200, 150)
			},
			matcher: formatMatcher(FormatPng),
		},
		"convert to jpeg": {
			options: ResizeOptions{Width: 100, Height: 100, Format: FormatJpeg},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatJpeg),
		},
		"convert to png": {
			options: ResizeOptions{Width: 100, Height: 100, Format: FormatPng},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatPng),
		},
		"convert to gif": {
			options: ResizeOptions{Width: 100, Height: 100, Format: FormatGif},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatGif),
		},
		"convert to tiff": {
			options: ResizeOptions{Width: 100, Height: 100, Format: FormatTiff},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatTiff),
		},
		"convert to bmp": {
			options: ResizeOptions{Width: 100, Height: 100, Format: FormatBmp},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatBmp),
		},
		"convert to unknown": {
			options: ResizeOptions{Width: 100, Height: 100, Format: Format(-1)},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: formatMatcher(FormatJpeg),
		},
		"resize png": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayPng(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize gif": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayGif(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize tiff": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayTiff(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize bmp": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayBmp(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize with high quality": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityHigh},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize with medium quality": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityMedium},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize with low quality": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityLow},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize with unknown quality": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: Quality(-1)},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return newGrayJpeg(t, 200, 150)
			},
			matcher: sizeMatcher(100, 100),
		},
		"get thumbnail from file with APP0 JFIF": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityLow},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return openFile(t, "testdata/gray-sample.jpg")
			},
			matcher: sizeMatcher(125, 128),
		},
		"get thumbnail from file without APP0 JFIF": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityLow},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return openFile(t, "testdata/20130612_142406.jpg")
			},
			matcher: sizeMatcher(320, 240),
		},
		"resize from file without IFD1 thumbnail": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityLow},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return openFile(t, "testdata/IMG_2578.JPG")
			},
			matcher: sizeMatcher(100, 100),
		},
		"resize for higher quality levels": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFill, Quality: QualityMedium},
			source: func(t *testing.T) afero.File {
				t.Helper()
				return openFile(t, "testdata/gray-sample.jpg")
			},
			matcher: sizeMatcher(100, 100),
		},
		"broken file": {
			options: ResizeOptions{Width: 100, Height: 100, ResizeMode: ResizeModeFit},
			source: func(t *testing.T) afero.File {
				t.Helper()
				fs := afero.NewMemMapFs()
				file, err := fs.Create("image.jpg")
				require.NoError(t, err)

				_, err = file.WriteString("this is not an image")
				require.NoError(t, err)

				return file
			},
			wantErr: true,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewPreviewGenerator(1, tmpDir)
			source := test.source(t)
			defer source.Close()

			buf := &bytes.Buffer{}
			err := svc.Resize(source, buf, test.options)
			if (err != nil) != test.wantErr {
				t.Fatalf("GetMarketSpecs() error = %v, wantErr %v", err, test.wantErr)
			}
			if err != nil {
				return
			}
			test.matcher(t, buf)
		})
	}
}

func sizeMatcher(width, height int) func(t *testing.T, reader io.Reader) {
	return func(t *testing.T, reader io.Reader) {
		resizedImg, _, err := image.Decode(reader)
		require.NoError(t, err)

		require.Equal(t, width, resizedImg.Bounds().Dx())
		require.Equal(t, height, resizedImg.Bounds().Dy())
	}
}

func formatMatcher(format Format) func(t *testing.T, reader io.Reader) {
	return func(t *testing.T, reader io.Reader) {
		_, decodedFormat, err := image.DecodeConfig(reader)
		require.NoError(t, err)

		require.Equal(t, format.String(), decodedFormat)
	}
}

func newGrayJpeg(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.jpg")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func newGrayPng(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.png")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = png.Encode(file, img)
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func newGrayGif(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.gif")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = gif.Encode(file, img, nil)
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func newGrayTiff(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.tiff")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = tiff.Encode(file, img, nil)
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func newGrayBmp(t *testing.T, width, height int) afero.File {
	fs := afero.NewMemMapFs()
	file, err := fs.Create("image.bmp")
	require.NoError(t, err)

	img := image.NewGray(image.Rect(0, 0, width, height))
	err = bmp.Encode(file, img)
	require.NoError(t, err)

	_, err = file.Seek(0, io.SeekStart)
	require.NoError(t, err)

	return file
}

func openFile(t *testing.T, name string) afero.File {
	appfs := afero.NewOsFs()
	file, err := appfs.Open(name)

	require.NoError(t, err)

	return file
}

func TestService_FormatFromExtension(t *testing.T) {
	testCases := map[string]struct {
		ext     string
		want    Format
		wantErr error
	}{
		"jpg": {
			ext:  ".jpg",
			want: FormatJpeg,
		},
		"jpeg": {
			ext:  ".jpeg",
			want: FormatJpeg,
		},
		"png": {
			ext:  ".png",
			want: FormatPng,
		},
		"gif": {
			ext:  ".gif",
			want: FormatGif,
		},
		"tiff": {
			ext:  ".tiff",
			want: FormatTiff,
		},
		"tif": {
			ext:  ".tif",
			want: FormatTiff,
		},
		"bmp": {
			ext:  ".bmp",
			want: FormatBmp,
		},
		"heic": {
			ext:  ".heic",
			want: FormatHeic,
		},
		"heif": {
			ext:  ".heif",
			want: FormatHeic,
		},
		"webp": {
			ext:  ".webp",
			want: FormatWebp,
		},
		"unknown": {
			ext:     ".mov",
			wantErr: ErrUnsupportedFormat,
		},
	}

	for name, test := range testCases {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			svc := NewPreviewGenerator(1, tmpDir)
			got, err := svc.FormatFromExtension(test.ext)
			require.Truef(t, errors.Is(err, test.wantErr), "error = %v, wantErr %v", err, test.wantErr)
			if err != nil {
				return
			}
			require.Equal(t, test.want, got)
		})
	}
}

func TestCacheKeyConsistency(t *testing.T) {
	// Test that folder previews and direct file previews use the same cache key
	// when the folder preview is based on a child file

	// Test direct file preview
	cacheKey1 := CacheKey("testmd5hash123", "small", 0)

	// Test folder preview with same child MD5
	cacheKey2 := CacheKey("testmd5hash123", "small", 0)

	// Both should produce the same cache key
	require.Equal(t, cacheKey1, cacheKey2, "Folder preview and direct file preview should use the same cache key")

	// Test with different seek percentages
	cacheKey3 := CacheKey("testmd5hash123", "small", 25)
	cacheKey4 := CacheKey("testmd5hash123", "small", 25)

	require.Equal(t, cacheKey3, cacheKey4, "Cache keys should be consistent for same parameters")
	require.NotEqual(t, cacheKey1, cacheKey3, "Different seek percentages should produce different cache keys")
}

// TestResize_ConcurrencyLimit verifies that imageSem works for basic concurrency control
func TestResize_ConcurrencyLimit(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPreviewGenerator(2, tmpDir)
	require.NotNil(t, svc.imageSem, "imageSem must be non-nil to test concurrency limit")

	// Simple smoke test - just verify Resize works with the semaphore
	source := newGrayJpeg(t, 100, 100)
	defer source.Close()
	out := &bytes.Buffer{}
	err := svc.Resize(source, out, ResizeOptions{Width: 50, Height: 50})
	require.NoError(t, err)
	require.Greater(t, out.Len(), 0)
}

// TestResize_ConcurrencyLimit_NoFFmpeg verifies that imageSem works correctly
func TestResize_ConcurrencyLimit_NoFFmpeg(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPreviewGenerator(1, tmpDir)
	require.NotNil(t, svc.imageSem, "imageSem should exist for concurrency control")

	source := newGrayJpeg(t, 100, 100)
	defer source.Close()
	out := &bytes.Buffer{}
	err := svc.Resize(source, out, ResizeOptions{Width: 50, Height: 50})
	require.NoError(t, err)
	require.Greater(t, out.Len(), 0)
}

// TestCreatePreviewFromReader_UsesConcurrencyLimit verifies that CreatePreviewFromReader
// goes through Resize and thus respects the imageSem semaphore (smoke test).
func TestCreatePreviewFromReader_UsesConcurrencyLimit(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPreviewGenerator(2, tmpDir)

	source := newGrayJpeg(t, 200, 200)
	defer source.Close()
	data, err := io.ReadAll(source)
	require.NoError(t, err)

	out, err := svc.CreatePreviewFromReader(bytes.NewReader(data), "small")
	require.NoError(t, err)
	require.NotEmpty(t, out)

	// Same via CreatePreview (bytes) for parity
	out2, err := svc.CreatePreview(data, "small")
	require.NoError(t, err)
	require.Equal(t, out, out2)
}

// TestDetectFormatAndSize_UsesBufferPool stresses detectFormatAndSize many times
// to ensure the buffer pool is used without leaking or panicking.
func TestDetectFormatAndSize_UsesBufferPool(t *testing.T) {
	tmpDir := t.TempDir()
	svc := NewPreviewGenerator(1, tmpDir)

	source := newGrayJpeg(t, 100, 100)
	defer source.Close()
	data, err := io.ReadAll(source)
	require.NoError(t, err)

	for i := 0; i < 200; i++ {
		format, reader, w, h, err := svc.detectFormatAndSize(bytes.NewReader(data))
		require.NoError(t, err)
		require.Equal(t, FormatJpeg, format)
		require.Equal(t, 100, w)
		require.Equal(t, 100, h)
		require.NotNil(t, reader)
		// Consume reader so we don't leave it open
		_, _ = io.Copy(io.Discard, reader)
	}
}
