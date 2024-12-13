//go:generate go-enum --sql --marshal --file $GOFILE
package img

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"io"

	"github.com/disintegration/imaging"
	"github.com/dsoprea/go-exif/v3"
)

// ErrUnsupportedFormat means the given image format is not supported.
var ErrUnsupportedFormat = errors.New("unsupported image format")

// Service handles image processing tasks.
type Service struct {
	sem chan struct{}
}

// New initializes the service with a specified number of workers (concurrency limit).
func New(workers int) *Service {
	return &Service{
		sem: make(chan struct{}, workers), // Buffered channel to limit concurrency.
	}
}

// acquire blocks until a worker is available or the context is canceled.
func (s *Service) acquire(ctx context.Context) error {
	select {
	case s.sem <- struct{}{}: // Reserve a worker.
		return nil
	case <-ctx.Done(): // Context canceled or deadline exceeded.
		return ctx.Err()
	}
}

// release frees up a worker slot.
func (s *Service) release() {
	select {
	case <-s.sem: // Free a worker slot.
	default:
		// Shouldn't happen, but guard against releasing more than acquired.
	}
}

// Resize processes image resizing based on raw byte data from an io.Reader.
func (s *Service) Resize(ctx context.Context, in io.Reader, width, height int, out io.Writer, options ...Option) error {
	if err := s.acquire(ctx); err != nil {
		return err
	}
	defer s.release()

	// Detect the format of the incoming image.
	format, wrappedReader, err := s.detectFormat(in)
	if err != nil {
		return err
	}

	// Apply options to configure resizing behavior.
	config := resizeConfig{
		format:     format,
		resizeMode: ResizeModeFit,
		quality:    QualityMedium,
	}
	for _, option := range options {
		option(&config)
	}

	// Handle thumbnail extraction for JPEG images with low quality setting.
	if config.quality == QualityLow && format == FormatJpeg {
		thm, newWrappedReader, errThm := getEmbeddedThumbnail(wrappedReader)
		wrappedReader = newWrappedReader
		if errThm == nil {
			// Write the extracted thumbnail directly to the output.
			_, err = out.Write(thm)
			if err == nil {
				return nil
			}
		}
	}

	// Decode the image using the wrapped reader with auto orientation.
	img, err := imaging.Decode(wrappedReader, imaging.AutoOrientation(true))
	if err != nil {
		return err
	}

	// Resize the image according to the specified mode and quality.
	switch config.resizeMode {
	case ResizeModeFill:
		img = imaging.Fill(img, width, height, imaging.Center, config.quality.resampleFilter())
	case ResizeModeFit:
		fallthrough //nolint:gocritic
	default:
		img = imaging.Fit(img, width, height, config.quality.resampleFilter())
	}

	// Encode and write the resized image to the output.
	return imaging.Encode(out, img, config.format.toImaging())
}

// detectFormat detects the image format based on raw data.
func (s *Service) detectFormat(in io.Reader) (Format, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)

	// Use image.DecodeConfig to get the format based on the raw byte data.
	_, imgFormat, err := image.DecodeConfig(r)
	if err != nil {
		return 0, nil, fmt.Errorf("%s: %w", err.Error(), ErrUnsupportedFormat)
	}

	// Parse the image format and map it to the custom Format enum.
	format, err := ParseFormat(imgFormat)
	if err != nil {
		return 0, nil, ErrUnsupportedFormat
	}

	// Return the detected format and a wrapped reader for further processing.
	return format, io.MultiReader(buf, in), nil
}

// getEmbeddedThumbnail attempts to extract embedded thumbnails from EXIF data.
func getEmbeddedThumbnail(in io.Reader) ([]byte, io.Reader, error) {
	buf := &bytes.Buffer{}
	r := io.TeeReader(in, buf)
	wrappedReader := io.MultiReader(buf, in)

	offset := 0
	offsets := []int{12, 30}
	head := make([]byte, 0xffff) //nolint:gomnd

	_, err := r.Read(head)
	if err != nil {
		return nil, wrappedReader, err
	}

	// Attempt to parse the EXIF header at various offsets.
	for _, offset = range offsets {
		if _, err = exif.ParseExifHeader(head[offset:]); err == nil {
			break
		}
	}

	// If EXIF header not found, return without a thumbnail.
	if err != nil {
		return nil, wrappedReader, err
	}

	// Extract the EXIF metadata and thumbnail if available.
	im, err := exifCommon.NewIfdMappingWithStandard()
	if err != nil {
		return nil, wrappedReader, err
	}

	_, index, err := exif.Collect(im, exif.NewTagIndex(), head[offset:])
	if err != nil {
		return nil, wrappedReader, err
	}

	// Check if the thumbnail is available in the EXIF data.
	ifd := index.RootIfd.NextIfd()
	if ifd == nil {
		return nil, wrappedReader, exif.ErrNoThumbnail
	}

	// Return the extracted thumbnail.
	thm, err := ifd.Thumbnail()
	return thm, wrappedReader, err
}
