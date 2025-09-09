package preview

import (
	"bytes"
	"context"
	"fmt"
	"image"
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
		return imaging.Lanczos
	case QualityMedium:
		return imaging.Box
	case QualityLow:
		return imaging.NearestNeighbor
	default:
		return imaging.Box
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
	default:
		return -1, ErrUnsupportedFormat
	}
}

type resizeConfig struct {
	format     Format
	resizeMode ResizeMode
	quality    Quality
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
	}
}

func (s *Service) Resize(in io.Reader, width, height int, out io.Writer, options ...Option) error {
	if err := s.acquire(context.Background()); err != nil {
		return err
	}
	defer s.release()

	format, wrappedReader, err := s.detectFormat(in)
	if err != nil {
		return err
	}

	config := resizeConfig{
		format:     format,
		resizeMode: ResizeModeFit,
		quality:    QualityMedium,
	}
	for _, option := range options {
		option(&config)
	}

	if format == FormatHeic {
		config.format = FormatJpeg
	}

	if config.quality == QualityLow && format == FormatJpeg {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil {
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	// For HEIC files, try without AutoOrientation first since it might not work properly
	img, err := imaging.Decode(wrappedReader)
	if err != nil {
		return err
	}

<<<<<<< HEAD
	// Note: For HEIC files processed via FFmpeg, orientation is handled automatically
=======
	// Try to detect if this is a HEIC file and manually handle rotation
	if format == FormatHeic {
		// EXIF shows 'Rotate 90 CW' (value 6), but applying 270° CW (90° CCW) to correct upside-down issue
		img = imaging.Rotate270(img) // 270° CW = 90° CCW
	}
>>>>>>> 586a76c5cae449e8a688e7baa23df5744877dda9

	switch config.resizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, width, height, imaging.Center, config.quality.resampleFilter())
	case ResizeModeFit:
		img = imaging.Fit(img, width, height, config.quality.resampleFilter())
	default:
		img = imaging.Fit(img, width, height, config.quality.resampleFilter())
	}

	return imaging.Encode(out, img, config.format.toImaging())
}

func (s *Service) detectFormat(in io.Reader) (Format, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)

	_, imgFormat, err := image.DecodeConfig(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", err.Error(), ErrUnsupportedFormat)
	}

	if imgFormat == "heif" {
		imgFormat = "heic"
	}

	format, err := ParseFormat(imgFormat)
	if err != nil {
		return 0, nil, ErrUnsupportedFormat
	}

	return format, io.MultiReader(buf, in), nil
}

func getEmbeddedThumbnail(in io.Reader) ([]byte, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)
	wrappedReader := io.MultiReader(buf, in)

	offsets := []int{12, 30}
	head := make([]byte, 0xffff)

	_, err := r.Read(head)
	if err != nil {
		return nil, wrappedReader, err
	}

	var offset int
	for _, offset = range offsets {
		if _, err = exif.ParseExifHeader(head[offset:]); err == nil {
			break
		}
	}

	if err != nil {
		return nil, wrappedReader, err
	}

	im, err := exifcommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, wrappedReader, err
	}

	_, index, err := exif.Collect(im, exif.NewTagIndex(), head[offset:])
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
