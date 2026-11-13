package ffmpeg

import (
	"bytes"
	"context"
	"image"
	"image/jpeg"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/kovidgoyal/imaging"
)

// HEICDisplayRotationReader reads the display rotation that a HEIC/HEIF/AVIF
// file's item properties (irot) ask for, without decoding it. The rotation is
// in degrees counter-clockwise, normalized to (-180, 180], which is the value
// ffmpeg prints as "Display Matrix: rotation of ..." for the same file. ok is
// false when the reader could not tell; the ffmpeg probe is used instead.
type HEICDisplayRotationReader func(path string) (rotation float64, ok bool)

// heicDisplayRotationReader is the header reader set by
// SetHEICDisplayRotationReader, or nil.
var heicDisplayRotationReader atomic.Pointer[HEICDisplayRotationReader]

// SetHEICDisplayRotationReader installs the reader HEICDisplayRotation tries
// before running ffmpeg. The gomedia package installs go-media's reader when it
// is loaded; it cannot be called from here directly because gomedia imports
// this package. Passing nil removes the reader.
func SetHEICDisplayRotationReader(fn HEICDisplayRotationReader) {
	if fn == nil {
		heicDisplayRotationReader.Store(nil)
		return
	}
	heicDisplayRotationReader.Store(&fn)
}

// HEICDisplayRotation returns the display rotation of heicPath and whether it is
// known. The installed header reader (go-media) answers first; the
// "ffmpeg -f null" stderr probe runs only when no reader is installed or the
// reader could not read the file.
func (s *Service) HEICDisplayRotation(ctx context.Context, heicPath string) (float64, bool) {
	if p := heicDisplayRotationReader.Load(); p != nil {
		if rotation, ok := (*p)(heicPath); ok {
			return rotation, true
		}
	}
	return s.GetHEICDisplayMatrixRotation(ctx, heicPath)
}

// GetHEICDisplayMatrixRotation returns the largest rotation (degrees) ffmpeg reports for a
// HEIC display matrix. The second return value is false when the probe could not run or
// parse stderr — callers must not treat that as “no rotation was applied”.
// HEICDisplayRotation is preferred; it only runs this probe as a fallback.
func (s *Service) GetHEICDisplayMatrixRotation(ctx context.Context, heicPath string) (float64, bool) {
	if s == nil {
		return 0, false
	}
	ffmpegPath := s.FFmpegPath()
	if ffmpegPath == "" || heicPath == "" {
		return 0, false
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-i", heicPath, "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return 0, false
	}

	rotation, ok := parseDisplayMatrixRotation(stderr.String())
	return rotation, ok
}

// parseDisplayMatrixRotation returns the largest-magnitude "Display Matrix"
// rotation in ffmpeg stderr, and whether any was found.
func parseDisplayMatrixRotation(stderr string) (float64, bool) {
	var best float64
	var found bool
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.Contains(line, "Display Matrix:") {
			continue
		}
		const prefix = "rotation of "
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		fields := strings.Fields(line[idx+len(prefix):])
		if len(fields) == 0 {
			continue
		}
		token := strings.TrimSuffix(fields[0], "degrees")
		deg, err := strconv.ParseFloat(token, 64)
		if err != nil {
			continue
		}
		found = true
		if math.Abs(deg) > math.Abs(best) {
			best = deg
		}
	}
	return best, found
}

// isNormalOrientation reports whether an EXIF orientation needs no change.
func isNormalOrientation(orientation string) bool {
	switch orientation {
	case "", "Horizontal (normal)", "Top-left", "1":
		return true
	default:
		return false
	}
}

// isPureRotationOrientation reports whether an EXIF orientation is a rotation
// without a mirror.
func isPureRotationOrientation(orientation string) bool {
	switch orientation {
	case "Rotate 90 CW", "Right-top", "6",
		"Rotate 270 CW", "Left-bottom", "8",
		"Rotate 180", "Bottom-right", "3":
		return true
	default:
		return false
	}
}

// orientationNeedsPostCorrection reports whether to rotate/flip JPEG bytes after ffmpeg decode.
func orientationNeedsPostCorrection(orientation string, displayRotation float64, displayKnown bool) bool {
	if isNormalOrientation(orientation) {
		return false
	}
	if isPureRotationOrientation(orientation) {
		if !displayKnown {
			// Probe failed: trust ffmpeg's display-matrix decode rather than risk double-rotation.
			return false
		}
		return displayRotation == 0
	}
	return true
}

// applyOrientationToJPEG rotates or flips JPEG bytes for an EXIF orientation.
// The input is returned unchanged when it cannot be decoded or re-encoded.
func applyOrientationToJPEG(jpegBytes []byte, orientation string) []byte {
	if len(jpegBytes) < 100 || isNormalOrientation(orientation) {
		return jpegBytes
	}

	img, err := imaging.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		return jpegBytes
	}

	var out image.Image
	switch orientation {
	case "Rotate 90 CW", "Right-top", "6":
		out = imaging.Rotate270(img)
	case "Rotate 180", "Bottom-right", "3":
		out = imaging.Rotate180(img)
	case "Rotate 270 CW", "Left-bottom", "8":
		out = imaging.Rotate90(img)
	case "Mirror horizontal", "Top-right", "2":
		out = imaging.FlipH(img)
	case "Mirror vertical", "Bottom-left", "4":
		out = imaging.FlipV(img)
	case "Mirror horizontal and rotate 270 CW", "Right-bottom", "7":
		out = imaging.Transverse(img)
	case "Mirror horizontal and rotate 90 CW", "Left-top", "5":
		out = imaging.Transpose(img)
	default:
		return jpegBytes
	}

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: 90}); err != nil {
		return jpegBytes
	}
	return buf.Bytes()
}
