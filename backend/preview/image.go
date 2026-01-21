package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"

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

type resizeConfig struct {
	format      Format
	resizeMode  ResizeMode
	quality     Quality
	jpegQuality int // JPEG encoding quality (1-100)
}

type Option func(*resizeConfig)

func WithFormat(format Format) Option {
	return func(config *resizeConfig) {
		config.format = format
	}
}

func WithMode(mode ResizeMode) Option {
	return func(config *resizeConfig) {
		config.resizeMode = mode
	}
}

func WithQuality(quality Quality) Option {
	return func(config *resizeConfig) {
		config.quality = quality
		config.jpegQuality = quality.jpegQuality()
	}
}

func (s *Service) Resize(in io.Reader, width, height int, out io.Writer, options ...Option) error {
	// First, detect format and get dimensions (lightweight operation)
	format, wrappedReader, imgWidth, imgHeight, err := s.detectFormatAndSize(in)
	if err != nil {
		return fmt.Errorf("failed to detect image format: %w", err)
	}

	// Use image service semaphore for image processing
	if s.imageService != nil {
		if acquireErr := s.imageService.Acquire(context.Background()); acquireErr != nil {
			return acquireErr
		}
		defer s.imageService.Release()
	}

	config := resizeConfig{
		format:      format,
		resizeMode:  ResizeModeFit,
		quality:     QualityMedium,
		jpegQuality: QualityMedium.jpegQuality(),
	}
	for _, option := range options {
		option(&config)
	}
	// Ensure JPEG quality is set based on quality setting
	if config.jpegQuality == 0 {
		config.jpegQuality = config.quality.jpegQuality()
	}

	if format == FormatHeic {
		config.format = FormatJpeg
	}

	// Try to use embedded EXIF thumbnail for JPEGs (saves MASSIVE memory and CPU)
	// This is especially beneficial for large images (e.g., 4000x3000 photos)
	if format == FormatJpeg && imgWidth > width*2 && imgHeight > height*2 {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil && len(thm) > 0 {
			// Decode the EXIF thumbnail to check its size
			thmImg, _, thmErr := image.Decode(bytes.NewReader(thm))
			if thmErr == nil {
				bounds := thmImg.Bounds()
				thmWidth := bounds.Dx()
				thmHeight := bounds.Dy()

				// If EXIF thumbnail is larger than or equal to requested size
				// and quality is Low, return it as-is (no need to resize down)
				// This is much faster than decoding the full image
				// For higher quality levels, we resize to exact dimensions for better quality
				if config.quality == QualityLow && thmWidth >= width && thmHeight >= height {
					if config.format == FormatJpeg {
						return jpeg.Encode(out, thmImg, &jpeg.Options{Quality: config.jpegQuality})
					}
					return imaging.Encode(out, thmImg, config.format.toImaging())
				}

				// If EXIF thumbnail is smaller but still reasonable (at least 40% of target),
				// resize it to the requested size
				minThumbSize := width * 40 / 100
				if thmWidth >= minThumbSize || thmHeight >= minThumbSize {
					var resizedImg image.Image
					switch config.resizeMode {
					case ResizeModeFill:
						resizedImg = imaging.Fill(thmImg, width, height, imaging.Center, config.quality.resampleFilter())
					case ResizeModeFit:
						resizedImg = imaging.Fit(thmImg, width, height, config.quality.resampleFilter())
					default:
						resizedImg = imaging.Fit(thmImg, width, height, config.quality.resampleFilter())
					}

					if config.format == FormatJpeg {
						return jpeg.Encode(out, resizedImg, &jpeg.Options{Quality: config.jpegQuality})
					}
					return imaging.Encode(out, resizedImg, config.format.toImaging())
				}
			}
		}
	}

	// For HEIC files, try without AutoOrientation first since it might not work properly
	// Try decoding with default settings (auto color space conversion)
	img, err := imaging.Decode(wrappedReader)
	if err != nil {
		// If decoding fails due to ICC profile issues (e.g., unsupported grayscale colorspace),
		// retry with color space conversion disabled
		if bytes.Contains([]byte(err.Error()), []byte("colorspace")) ||
			bytes.Contains([]byte(err.Error()), []byte("ICC profile")) {
			// Reset reader to beginning
			if seeker, ok := wrappedReader.(io.Seeker); ok {
				_, _ = seeker.Seek(0, io.SeekStart)
			}
			// Try again without color space conversion
			img, err = imaging.Decode(wrappedReader, imaging.ColorSpace(imaging.NO_CHANGE_OF_COLORSPACE))
			if err != nil {
				return fmt.Errorf("failed to decode image: %w", err)
			}
		} else {
			return fmt.Errorf("failed to decode image: %w", err)
		}
	}

	// Note: For HEIC files processed via FFmpeg, orientation is handled automatically

	// Get current image dimensions
	bounds := img.Bounds()
	currentWidth := bounds.Dx()
	currentHeight := bounds.Dy()

	// For very large images (>8x target size), use two-pass resize for better performance
	// First pass: resize to 2x target, Second pass: resize to target
	// This is faster and uses less memory than single-pass resize
	if currentWidth > width*8 || currentHeight > height*8 {
		intermediateWidth := width * 2
		intermediateHeight := height * 2
		// Use fast filter for first pass
		img = imaging.Fit(img, intermediateWidth, intermediateHeight, imaging.Box)
	}

	// Optimize resampling filter based on size and quality
	// For small thumbnails (< 512px), use faster filters
	resampleFilter := config.quality.resampleFilter()
	maxDimension := width
	if height > maxDimension {
		maxDimension = height
	}
	if maxDimension <= 256 {
		// For very small thumbnails, use fastest filter
		resampleFilter = imaging.Box
	} else if maxDimension <= 512 && config.quality != QualityHigh {
		// For medium thumbnails with non-high quality, use Box
		resampleFilter = imaging.Box
	}

	switch config.resizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, width, height, imaging.Center, resampleFilter)
	case ResizeModeFit:
		img = imaging.Fit(img, width, height, resampleFilter)
	default:
		img = imaging.Fit(img, width, height, resampleFilter)
	}

	// Use optimized JPEG encoding with quality control for better performance
	if config.format == FormatJpeg {
		// Further optimize quality for small thumbnails
		jpegQuality := config.jpegQuality
		return jpeg.Encode(out, img, &jpeg.Options{Quality: jpegQuality})
	}

	return imaging.Encode(out, img, config.format.toImaging())
}

// detectFormatAndSize detects format and dimensions without full decode
func (s *Service) detectFormatAndSize(in io.Reader) (Format, io.Reader, int, int, error) {
	const maxHeaderSize = 64 * 1024 // 64KB should be enough for format detection

	// Try to work with seekable readers efficiently
	if seeker, ok := in.(io.Seeker); ok {
		originalPos, err := seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			// Seek to start
			if _, err := seeker.Seek(0, io.SeekStart); err == nil {
				headerBuf := make([]byte, maxHeaderSize)
				n, readErr := in.Read(headerBuf)
				// Handle EOF gracefully (file smaller than header size)
				if readErr == nil || readErr == io.EOF {
					if n >= 2 {
						headerBuf = headerBuf[:n]
						isFullFile := (readErr == io.EOF) || (n < maxHeaderSize)

						// Try magic byte detection first (fastest)
						format := s.detectFormatFromMagicBytes(headerBuf, n)

						// Try to decode config for dimensions
						reader := bytes.NewReader(headerBuf)
						config, imgFormat, configErr := image.DecodeConfig(reader)

						if configErr == nil {
							// Successfully got config
							if format < 0 {
								format = s.parseFormat(imgFormat)
							}
							if format >= 0 {
								if isFullFile {
									// Small file - use the buffer we already have
									return format, bytes.NewReader(headerBuf), config.Width, config.Height, nil
								}
								// Large file - seek back and use original reader
								_, _ = seeker.Seek(originalPos, io.SeekStart)
								return format, in, config.Width, config.Height, nil
							}
						}

						// Restore position and fall through
						_, _ = seeker.Seek(originalPos, io.SeekStart)
					}
				}
			}
		}
	}

	// Fallback: read all (for non-seekable or if header detection failed)
	allData, err := io.ReadAll(in)
	if err != nil {
		return -1, nil, 0, 0, fmt.Errorf("failed to read image data: %w", err)
	}
	if len(allData) == 0 {
		return -1, nil, 0, 0, fmt.Errorf("image data is empty: %w", ErrUnsupportedFormat)
	}

	reader := bytes.NewReader(allData)
	config, imgFormat, err := image.DecodeConfig(reader)
	if err != nil {
		return -1, nil, 0, 0, fmt.Errorf("image.DecodeConfig failed: %w", ErrUnsupportedFormat)
	}

	format := s.parseFormat(imgFormat)
	if format < 0 {
		return -1, nil, 0, 0, fmt.Errorf("unsupported image format '%s': %w", imgFormat, ErrUnsupportedFormat)
	}

	return format, bytes.NewReader(allData), config.Width, config.Height, nil
}

// detectFormatFromMagicBytes detects image format from magic bytes in file header
// Returns -1 if no format is detected (since FormatJpeg = 0)
func (s *Service) detectFormatFromMagicBytes(header []byte, n int) Format {
	if n < 2 {
		return -1
	}

	// JPEG: FF D8 (the third byte can vary, so we only check first two)
	if header[0] == 0xFF && header[1] == 0xD8 {
		return FormatJpeg
	}

	// PNG: 89 50 4E 47 0D 0A 1A 0A
	if n >= 8 && header[0] == 0x89 && header[1] == 0x50 && header[2] == 0x4E && header[3] == 0x47 {
		return FormatPng
	}

	// GIF: GIF87a or GIF89a
	if n >= 6 && (string(header[0:6]) == "GIF87a" || string(header[0:6]) == "GIF89a") {
		return FormatGif
	}

	// BMP: BM (42 4D)
	if n >= 2 && header[0] == 0x42 && header[1] == 0x4D {
		return FormatBmp
	}

	// TIFF: Little-endian (49 49 2A 00) or Big-endian (4D 4D 00 2A)
	if n >= 4 {
		if (header[0] == 0x49 && header[1] == 0x49 && header[2] == 0x2A && header[3] == 0x00) ||
			(header[0] == 0x4D && header[1] == 0x4D && header[2] == 0x00 && header[3] == 0x2A) {
			return FormatTiff
		}
	}

	// WebP: RIFF...WEBP (starts with RIFF, contains WEBP at offset 8)
	if n >= 12 && string(header[0:4]) == "RIFF" && string(header[8:12]) == "WEBP" {
		return FormatWebp
	}

	// HEIC/HEIF: ftyp box (starts with 4-byte size, then 'ftyp', then brand)
	// Common brands: heic, heif, mif1
	if n >= 12 {
		// Check for ftyp at offset 4
		if string(header[4:8]) == "ftyp" {
			// Check for HEIC/HEIF brands
			brand := string(header[8:12])
			if brand == "heic" || brand == "heif" || brand == "mif1" {
				return FormatHeic
			}
		}
	}

	// Netpbm formats: P1-P7 (ASCII) or binary variants
	if n >= 2 && header[0] == 'P' {
		switch header[1] {
		case '1', '4': // PBM (P1=ASCII, P4=binary)
			return FormatPbm
		case '2', '5': // PGM (P2=ASCII, P5=binary)
			return FormatPgm
		case '3', '6': // PPM (P3=ASCII, P6=binary)
			return FormatPpm
		case '7': // PAM
			return FormatPam
		}
	}

	return -1 // No format detected
}

// parseFormat parses the image format string and returns the Format enum
func (s *Service) parseFormat(imgFormat string) Format {
	if imgFormat == "heif" {
		imgFormat = "heic"
	}
	// Handle case variations - image.DecodeConfig might return "jpeg" or "JPEG"
	imgFormat = strings.ToLower(imgFormat)

	// Manual mapping to ensure compatibility (bypasses generated enum ParseFormat)
	switch imgFormat {
	case "jpeg", "jpg":
		return FormatJpeg
	case "png":
		return FormatPng
	case "gif":
		return FormatGif
	case "tiff", "tif":
		return FormatTiff
	case "bmp":
		return FormatBmp
	case "heic", "heif":
		return FormatHeic
	case "webp":
		return FormatWebp
	case "pbm":
		return FormatPbm
	case "pgm":
		return FormatPgm
	case "ppm":
		return FormatPpm
	case "pam":
		return FormatPam
	default:
		return -1
	}
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
				// Read error - fallback to reading all
				_, _ = seeker.Seek(originalPos, io.SeekStart)
				allData, _ := io.ReadAll(in)
				headerBuf = allData
				wrappedReader = bytes.NewReader(allData)
			}
		} else {
			// Seek failed, read all
			allData, _ := io.ReadAll(in)
			headerBuf = allData
			wrappedReader = bytes.NewReader(allData)
		}
	} else {
		// Non-seekable reader - must read and combine
		buf := make([]byte, maxExifSize)
		n, err := io.ReadFull(in, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return nil, nil, err
		}
		headerBuf = buf[:n]
		// Read remaining data and combine
		remaining, _ := io.ReadAll(in)
		combined := append(headerBuf, remaining...)
		wrappedReader = bytes.NewReader(combined)
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
