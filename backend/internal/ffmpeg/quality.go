package ffmpeg

import (
	"strconv"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
)

const (
	// defaultLegacyMJPEGQScale is the historical 1–10 -q:v value used when no quality is given.
	defaultLegacyMJPEGQScale = 5
)

// frameQualityForLegacyMJPEGQScale maps the pre–go-ffmpeg v1.5.4 scale where quality 1–10 was
// passed directly to ffmpeg -q:v (lower is better) onto the 1–100 frame quality scale.
func frameQualityForLegacyMJPEGQScale(legacy int) int {
	if legacy < 1 {
		legacy = 1
	}
	if legacy > 10 {
		legacy = 10
	}
	if legacy == 1 {
		// -q:v 1 is not reachable on the new scale; quality 100 is the best MJPEG output.
		return 100
	}
	target := strconv.Itoa(legacy)
	var matches []int
	for q := 1; q <= 100; q++ {
		if encode.MJPEGQualityQScale(q) == target {
			matches = append(matches, q)
		}
	}
	if len(matches) == 0 {
		return encode.DefaultFrameQuality
	}
	idx := (legacy - 1) * (len(matches) - 1) / 9
	return matches[idx]
}

// parseQuality interprets a quality string for go-ffmpeg ConvertHEIC / MJPEG encode.
// Values 1–10 are treated as the legacy -q:v scale (only used internally by preview today).
// Values 11–100 use the new frame quality scale. Invalid or empty input keeps the historical default.
func parseQuality(quality string) int {
	if quality == "" {
		return frameQualityForLegacyMJPEGQScale(defaultLegacyMJPEGQScale)
	}
	n, err := strconv.Atoi(quality)
	if err != nil || n < 1 {
		return frameQualityForLegacyMJPEGQScale(defaultLegacyMJPEGQScale)
	}
	if n <= 10 {
		return frameQualityForLegacyMJPEGQScale(n)
	}
	if n > 100 {
		return 100
	}
	return n
}
