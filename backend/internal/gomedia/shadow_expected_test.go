package gomedia

import (
	"image/color"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/stretchr/testify/require"
)

// Different frames of the same video are an expected difference, logged at info.
func TestThumbnailDiffFrameChoiceIsExpected(t *testing.T) {
	a := solidJPEG(t, 64, 48, color.Gray{Y: 100})
	b := solidJPEG(t, 64, 48, color.Gray{Y: 120})
	d := thumbnailDiff(a, b)
	require.Len(t, d, 1)
	require.True(t, strings.HasPrefix(d[0], expectedDiffPrefix), d[0])
}

// A black frame on one side is a real bug, so it must stay a warning.
func TestThumbnailDiffBlackFrameIsReal(t *testing.T) {
	black := solidJPEG(t, 64, 48, color.Gray{Y: 16})
	bright := solidJPEG(t, 64, 48, color.Gray{Y: 170})
	d := thumbnailDiff(black, bright)
	require.Len(t, d, 1)
	require.False(t, strings.HasPrefix(d[0], expectedDiffPrefix), d[0])
	require.Contains(t, d[0], "mean luma")
}

// Moderate luma gaps below thumbHardLumaDiff are expected frame choice.
func TestThumbnailDiffModerateLumaIsExpected(t *testing.T) {
	a := solidJPEG(t, 64, 48, color.Gray{Y: 80})
	b := solidJPEG(t, 64, 48, color.Gray{Y: 130})
	d := thumbnailDiff(a, b)
	require.Len(t, d, 1)
	require.True(t, strings.HasPrefix(d[0], expectedDiffPrefix), d[0])
}

// Very large luma differences are real even when neither side is black.
func TestThumbnailDiffLargeLumaIsReal(t *testing.T) {
	a := solidJPEG(t, 64, 48, color.Gray{Y: 60})
	b := solidJPEG(t, 64, 48, color.Gray{Y: 220})
	d := thumbnailDiff(a, b)
	require.Len(t, d, 1)
	require.False(t, strings.HasPrefix(d[0], expectedDiffPrefix), d[0])
}

// go-media lists sidecars in the same order and with the same language rule as
// the directory scan, so the shadow comparison matches.
func TestExternalSubtitlesFromGoMedia(t *testing.T) {
	withGate(t, false)
	names := []string{"video.srt", "video.en.srt", "video.sbv", "video.ass", "other.srt"}

	got, ok := ExternalSubtitles("video.mp4", names, nil)
	require.True(t, ok)
	require.Equal(t, []utils.SubtitleTrack{
		{Name: "video.srt", Codec: "srt"},
		{Name: "video.en.srt", Language: "en", Codec: "srt"},
		{Name: "video.ass", Codec: "ass"},
	}, got)
}
