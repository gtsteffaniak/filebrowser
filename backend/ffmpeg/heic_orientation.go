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

	"github.com/kovidgoyal/imaging"
)

// GetHEICDisplayMatrixRotation returns the largest non-zero rotation (degrees) ffmpeg
// reports for a HEIC file's display matrix. Zero means no rotation was applied on decode.
func (s *Service) GetHEICDisplayMatrixRotation(ctx context.Context, heicPath string) float64 {
	if s == nil {
		return 0
	}
	ffmpegPath := s.FFmpegPath()
	if ffmpegPath == "" || heicPath == "" {
		return 0
	}

	cmd := exec.CommandContext(ctx, ffmpegPath, "-hide_banner", "-i", heicPath, "-f", "null", "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	_ = cmd.Run()

	return parseDisplayMatrixRotation(stderr.String())
}

func parseDisplayMatrixRotation(stderr string) float64 {
	var best float64
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.Contains(line, "Display Matrix:") {
			continue
		}
		const prefix = "rotation of "
		idx := strings.Index(line, prefix)
		if idx < 0 {
			continue
		}
		token := strings.Fields(line[idx+len(prefix):])[0]
		token = strings.TrimSuffix(token, "degrees")
		deg, err := strconv.ParseFloat(token, 64)
		if err != nil {
			continue
		}
		if math.Abs(deg) > math.Abs(best) {
			best = deg
		}
	}
	return best
}

func isNormalOrientation(orientation string) bool {
	switch orientation {
	case "", "Horizontal (normal)", "Top-left", "1":
		return true
	default:
		return false
	}
}

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
// ffmpeg applies QuickTime/display-matrix rotation on decode; skip Go correction when that
// already satisfied a pure rotation. Mirrors and other transforms still need Go when the
// display matrix is zero.
func orientationNeedsPostCorrection(orientation string, displayRotation float64) bool {
	if isNormalOrientation(orientation) {
		return false
	}
	if displayRotation != 0 && isPureRotationOrientation(orientation) {
		return false
	}
	return true
}

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
