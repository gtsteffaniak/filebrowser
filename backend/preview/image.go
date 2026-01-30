package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"strings"
	"time"

	exif "github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	"github.com/gtsteffaniak/go-logger/logger"
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
	
	// Acquire appropriate semaphore based on file size
	// Large files (>=8MB) use imageLargeSem if available, otherwise fall back to imageSem
	// Small files (<8MB) use imageSem
	const largeFileSizeThreshold = 8 * 1024 * 1024 // 8MB
	
	if fileSize >= largeFileSizeThreshold && s.imageLargeSem != nil {
		// Large file path
		select {
		case s.imageLargeSem <- struct{}{}:
			defer func() { <-s.imageLargeSem }()
		case <-context.Background().Done():
			return context.Background().Err()
		}
	} else {
		// Small file path or fallback
		select {
		case s.imageSem <- struct{}{}:
			defer func() { <-s.imageSem }()
		case <-context.Background().Done():
			return context.Background().Err()
		}
	}

	// Detect format and dimensions (lightweight for seekable readers; may ReadAll for non-seekable).
	detectedFormat, wrappedReader, imgWidth, imgHeight, err := s.detectFormatAndSize(in)
	if err != nil {
		return fmt.Errorf("failed to detect image format: %w", err)
	}
	
	// Only set format if not explicitly specified
	if opts.Format == 0 {
		opts.Format = detectedFormat
	}

	if detectedFormat == FormatHeic {
		opts.Format = FormatJpeg
	}

	// Try to use embedded EXIF thumbnail for JPEGs (saves MASSIVE memory and CPU)
	// This is especially beneficial for large images (e.g., 4000x3000 photos)
	if detectedFormat == FormatJpeg && imgWidth > opts.Width*2 && imgHeight > opts.Height*2 {
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
				if opts.Quality == QualityLow && thmWidth >= opts.Width && thmHeight >= opts.Height {
					if opts.Format == FormatJpeg {
						return jpeg.Encode(out, thmImg, &jpeg.Options{Quality: opts.JpegQuality})
					}
					return imaging.Encode(out, thmImg, opts.Format.toImaging())
				}

				// If EXIF thumbnail is smaller but still reasonable (at least 40% of target),
				minThumbSize := opts.Width * 40 / 100
				if thmWidth >= minThumbSize || thmHeight >= minThumbSize {
					var resizedImg image.Image
					switch opts.ResizeMode {
					case ResizeModeFill:
						resizedImg = imaging.Fill(thmImg, opts.Width, opts.Height, imaging.Center, opts.Quality.resampleFilter())
					case ResizeModeFit:
						resizedImg = imaging.Fit(thmImg, opts.Width, opts.Height, opts.Quality.resampleFilter())
					default:
						resizedImg = imaging.Fit(thmImg, opts.Width, opts.Height, opts.Quality.resampleFilter())
					}

					if opts.Format == FormatJpeg {
						return jpeg.Encode(out, resizedImg, &jpeg.Options{Quality: opts.JpegQuality})
					}
					return imaging.Encode(out, resizedImg, opts.Format.toImaging())
				}
			}
		}
	}

	// For HEIC files, try without AutoOrientation first since it might not work properly
	// Set a timeout for decode operations to prevent runaway processes
	// Use a channel to signal completion and handle timeout
	type decodeResult struct {
		img image.Image
		err error
	}
	
	decodeChan := make(chan decodeResult, 1)
	decodeTimeout := 5 * time.Second
	
	go func() {
		// Try decoding with default settings (auto color space conversion)
		img, err := imaging.Decode(wrappedReader)
		if err != nil {
			// If decoding fails due to ICC profile issues, retry without color space conversion
			if bytes.Contains([]byte(err.Error()), []byte("colorspace")) ||
				bytes.Contains([]byte(err.Error()), []byte("ICC profile")) {
				// Reset reader to beginning
				if seeker, ok := wrappedReader.(io.Seeker); ok {
					_, _ = seeker.Seek(0, io.SeekStart)
				}
				// Try again without color space conversion
				img, err = imaging.Decode(wrappedReader, imaging.ColorSpace(imaging.NO_CHANGE_OF_COLORSPACE))
			}
		}
		decodeChan <- decodeResult{img: img, err: err}
	}()
	
	var img image.Image
	select {
	case result := <-decodeChan:
		if result.err != nil {
			return fmt.Errorf("failed to decode image: %w", result.err)
		}
		img = result.img
	case <-time.After(decodeTimeout):
		return fmt.Errorf("image decode timeout exceeded (%v) for %v image %dx%d", 
			decodeTimeout, detectedFormat, imgWidth, imgHeight)
	}

	// Note: For HEIC files processed via FFmpeg, orientation is handled automatically

	// Use the quality setting requested by caller
	resampleFilter := opts.Quality.resampleFilter()

	switch opts.ResizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, opts.Width, opts.Height, imaging.Center, resampleFilter)
	case ResizeModeFit:
		img = imaging.Fit(img, opts.Width, opts.Height, resampleFilter)
	default:
		img = imaging.Fit(img, opts.Width, opts.Height, resampleFilter)
	}

	// Use optimized JPEG encoding with quality control for better performance
	var encodeErr error
	if opts.Format == FormatJpeg {
		// Further optimize quality for small thumbnails
		jpegQuality := opts.JpegQuality
		encodeErr = jpeg.Encode(out, img, &jpeg.Options{Quality: jpegQuality})
	} else {
		encodeErr = imaging.Encode(out, img, opts.Format.toImaging())
	}
	
	return encodeErr
}

// detectFormatAndSize detects format and dimensions without full decode
func (s *Service) detectFormatAndSize(in io.Reader) (Format, io.Reader, int, int, error) {
	// Progressive buffer sizes: 128KB first, then 512KB if needed
	headerSizes := []int64{
		128 * 1024, // 128KB - handles most images
		512 * 1024, // 512KB - fallback for images with extensive EXIF/metadata
	}

	// Try to work with seekable readers efficiently
	if seeker, ok := in.(io.Seeker); ok {
		originalPos, err := seeker.Seek(0, io.SeekCurrent)
		if err == nil {
			if _, err := seeker.Seek(0, io.SeekStart); err == nil {
				buf := getBuffer()
				defer putBuffer(buf)

				// Try progressively larger buffer sizes
				for attemptIndex, headerSize := range headerSizes {
					// Reset buffer and seek position for each attempt
					buf.Reset()
					_, _ = seeker.Seek(0, io.SeekStart)

					limitedReader := io.LimitReader(in, headerSize)
					n64, readErr := buf.ReadFrom(limitedReader)
					n := int(n64)
					if readErr == nil || readErr == io.EOF {
						if n >= 2 {
							headerBytes := buf.Bytes()
							isFullFile := (readErr == io.EOF) || (n < int(headerSize))

							format := s.detectFormatFromMagicBytes(headerBytes, n)
							reader := bytes.NewReader(headerBytes)
							config, imgFormat, configErr := image.DecodeConfig(reader)

							if configErr == nil {
								if format < 0 {
									format = s.parseFormat(imgFormat)
								}
								if format >= 0 {
									// Log retry attempts to track if 512KB is needed
									if attemptIndex > 0 {
										logger.Debugf("format detection required %dKB buffer (retry #%d)", headerSize/1024, attemptIndex)
									}
									if isFullFile {
										// Small file - copy so we don't hold pool memory after return
										copyBuf := make([]byte, n)
										copy(copyBuf, headerBytes)
										return format, bytes.NewReader(copyBuf), config.Width, config.Height, nil
									}
									_, _ = seeker.Seek(originalPos, io.SeekStart)
									return format, in, config.Width, config.Height, nil
								}
								// Unsupported format - don't retry
								break
							} else {
								// Check if this was an "unexpected EOF" - if so, retry with larger buffer
								errMsg := configErr.Error()
								isUnexpectedEOF := strings.Contains(errMsg, "unexpected EOF")

								if isUnexpectedEOF && attemptIndex < len(headerSizes)-1 {
									// Retry with larger buffer
									continue
								}
								// Non-EOF error or exhausted all buffer sizes (both 128KB and 512KB failed)
								logger.Warningf("format detection failed after trying up to %dKB: %v", headerSize/1024, configErr)
								break
							}
						}
					}
				}
				_, _ = seeker.Seek(originalPos, io.SeekStart)
			}
		}
	}

	// Fallback disabled for memory debugging: avoid ReadAll so we can see if full-file reads were the issue.
	// If reader is not seekable, or header detection failed, we fail instead of loading the whole file.
	if _, ok := in.(io.Seeker); ok {
		return -1, nil, 0, 0, fmt.Errorf("could not detect image format from header (seekable reader): %w", ErrUnsupportedFormat)
	}
	return -1, nil, 0, 0, fmt.Errorf("image format detection requires seekable reader (e.g. *os.File or *bytes.Reader): %w", ErrUnsupportedFormat)
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
