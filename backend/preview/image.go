package preview

import (
	"fmt"
	"image"
	"image/jpeg"
	"io"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	"github.com/kovidgoyal/imaging"
)

// Format is an image file format.
/*
ENUM(
jpeg
png
gif
tiff
bmp
heic
webp
pbm
pgm
ppm
pam
)
*/
type Format int

func (x Format) toImaging() imaging.Format {
	switch x {
	case FormatJpeg:
		return imaging.JPEG
	case FormatPng:
		return imaging.PNG
	case FormatGif:
		return imaging.GIF
	case FormatTiff:
		return imaging.TIFF
	case FormatBmp:
		return imaging.BMP
	case FormatHeic:
		return imaging.JPEG
	case FormatWebp:
		return imaging.WEBP
	case FormatPbm:
		return imaging.PBM
	case FormatPgm:
		return imaging.PGM
	case FormatPpm:
		return imaging.PPM
	case FormatPam:
		return imaging.PAM
	default:
		return imaging.JPEG
	}
}

/*
ENUM(
high
medium
low
)
*/
type Quality int

func (x Quality) resampleFilter() imaging.ResampleFilter {
	switch x {
	case QualityHigh:
		// Use CatmullRom instead of Lanczos - faster with similar quality for thumbnails
		return imaging.CatmullRom
	case QualityMedium:
		return imaging.Box
	case QualityLow:
		return imaging.NearestNeighbor
	default:
		return imaging.Box
	}
}

// jpegQuality returns JPEG quality (1-100) based on Quality setting
func (x Quality) jpegQuality() int {
	switch x {
	case QualityHigh:
		return 85 // Good quality, faster than 95+
	case QualityMedium:
		return 75 // Balanced quality/speed
	case QualityLow:
		return 65 // Lower quality, faster encoding
	default:
		return 75
	}
}

/*
ENUM(
fit
fill
)
*/
type ResizeMode int

func (s *Service) FormatFromExtension(ext string) (Format, error) {
	// heic is not supported by imaging, so we return FormatHeic
	switch ext {
	case ".heic", ".heif":
		return FormatHeic, nil
	}

	format, err := imaging.FormatFromExtension(ext)
	if err != nil {
		return -1, ErrUnsupportedFormat
	}
	switch format {
	case imaging.JPEG:
		return FormatJpeg, nil
	case imaging.PNG:
		return FormatPng, nil
	case imaging.GIF:
		return FormatGif, nil
	case imaging.TIFF:
		return FormatTiff, nil
	case imaging.BMP:
		return FormatBmp, nil
	case imaging.WEBP:
		return FormatWebp, nil
	case imaging.PBM:
		return FormatPbm, nil
	case imaging.PGM:
		return FormatPgm, nil
	case imaging.PPM:
		return FormatPpm, nil
	case imaging.PAM:
		return FormatPam, nil
	default:
		return -1, ErrUnsupportedFormat
	}
}

type ResizeOptions struct {
	Width       int
	Height      int
	Format      Format
	ResizeMode  ResizeMode
	Quality     Quality
	JpegQuality int // JPEG encoding quality (1-100), 0 means use Quality default
}

func (s *Service) Resize(in io.Reader, out io.Writer, opts ResizeOptions) error {
	return s.ResizeWithSize(in, out, 0, opts)
}

// ResizeWithSize resizes an image with file size information for appropriate semaphore selection
func (s *Service) ResizeWithSize(in io.Reader, out io.Writer, fileSize int64, opts ResizeOptions) error {
	// Set defaults
	if opts.ResizeMode == 0 {
		opts.ResizeMode = ResizeModeFit
	}
	if opts.Quality == 0 {
		opts.Quality = QualityMedium
	}
	if opts.JpegQuality == 0 {
		opts.JpegQuality = opts.Quality.jpegQuality()
	}

	// Skip format detection - format is already known (FormatJpeg from CreatePreviewFromReaderWithSize)
	// This avoids unnecessary I/O and DecodeConfig issues with corrupted files
	var wrappedReader = in
	var err error

	if opts.Format == FormatHeic {
		opts.Format = FormatJpeg
	}

	// Try to use embedded EXIF thumbnail for JPEGs (only for low quality to keep it simple)
	if opts.Format == FormatJpeg && opts.Quality == QualityLow {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil && len(thm) > 0 {
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	// Decode the image - try imaging library first, fall back to format-specific decoder if it fails
	img, err := imaging.Decode(wrappedReader, imaging.AutoOrientation(true))
	if err != nil {
		// Imaging library failed, try format-specific standard decoder as fallback
		// Reset reader if possible
		if seeker, ok := wrappedReader.(io.Seeker); ok {
			_, _ = seeker.Seek(0, io.SeekStart)
		}

		// Use format-specific decoder (more reliable than image.Decode auto-detection)
		if opts.Format == FormatJpeg {
			img, err = jpeg.Decode(wrappedReader)
			if err != nil {
				// Return error with JPEG-specific terms so FFmpeg fallback can catch it
				return fmt.Errorf("failed to decode image: %w", err)
			}
		} else {
			// For other formats, try auto-detection
			img, _, err = image.Decode(wrappedReader)
			if err != nil {
				return fmt.Errorf("failed to decode image: %w", err)
			}
		}
	}

	// Resize
	resampleFilter := opts.Quality.resampleFilter()
	switch opts.ResizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, opts.Width, opts.Height, imaging.Center, resampleFilter)
	case ResizeModeFit:
		fallthrough
	default:
		img = imaging.Fit(img, opts.Width, opts.Height, resampleFilter)
	}

	// Encode
	if opts.Format == FormatJpeg {
		return jpeg.Encode(out, img, &jpeg.Options{Quality: opts.JpegQuality})
	}
	return imaging.Encode(out, img, opts.Format.toImaging())
}

func getEmbeddedThumbnail(in io.Reader) ([]byte, io.Reader, error) {
	// Optimize memory: only read enough bytes for EXIF parsing (max 64KB)
	// EXIF data is typically in the first 64KB of JPEG files
	const maxExifSize = 0xffff // 64KB

	var headerBuf []byte
	var wrappedReader io.Reader

	// Check if reader is seekable - if so, we can peek and seek back (most efficient)
	if seeker, ok := in.(io.Seeker); ok {
		originalPos, err := seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			// Use buffer pool for temporary storage
			buf := getBuffer()
			defer putBuffer(buf)

			// Read just the header
			limitedReader := io.LimitReader(in, maxExifSize)
			_, readErr := buf.ReadFrom(limitedReader)
			if readErr == nil || readErr == io.EOF {
				// Copy to permanent storage for EXIF parsing
				headerBuf = make([]byte, buf.Len())
				copy(headerBuf, buf.Bytes())
				// Seek back to start for main processing
				_, _ = seeker.Seek(originalPos, io.SeekStart)
				wrappedReader = in
			} else {
				// Read error - cannot extract thumbnail, return error to fall back to full decode
				_, _ = seeker.Seek(originalPos, io.SeekStart)
				return nil, in, fmt.Errorf("failed to read header for thumbnail extraction: %w", readErr)
			}
		} else {
			// Seek failed - cannot extract thumbnail without seeking, return error to fall back to full decode
			return nil, in, fmt.Errorf("reader is not seekable, cannot extract thumbnail")
		}
	} else {
		// Non-seekable reader - cannot extract thumbnail without loading entire file
		// Return error to fall back to full decode rather than loading everything into memory
		return nil, in, fmt.Errorf("reader is not seekable, cannot extract thumbnail without loading entire file")
	}

	// Try to find EXIF header
	offsets := []int{12, 30}

	var offset int
	var exifErr error
	for _, offset = range offsets {
		if offset < len(headerBuf) {
			if _, exifErr = exif.ParseExifHeader(headerBuf[offset:]); exifErr == nil {
				break
			}
		}
	}

	if exifErr != nil {
		return nil, wrappedReader, exifErr
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, wrappedReader, err
	}

	_, index, err := exif.Collect(im, exif.NewTagIndex(), headerBuf[offset:])
	if err != nil {
		return nil, wrappedReader, err
	}

	ifd := index.RootIfd.NextIfd()
	if ifd == nil {
		return nil, wrappedReader, exif.ErrNoThumbnail
	}

	thm, err := ifd.Thumbnail()
	return thm, wrappedReader, err
}

// CreateThumbnail decodes an image and creates a fixed-size thumbnail.
func CreateThumbnail(rawData io.Reader, width, height int) (image.Image, error) {
	img, _, err := image.Decode(rawData)
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}
	thumb := imaging.Fit(img, width, height, imaging.Lanczos)
	return thumb, nil
}
