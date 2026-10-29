package gomedia

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/go-media/pkg/media"
	"github.com/stretchr/testify/require"
)

func solidJPEG(t *testing.T, w, h int, c color.Gray) []byte {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetGray(x, y, c)
		}
	}
	var buf bytes.Buffer
	require.NoError(t, jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}))
	return buf.Bytes()
}

func TestProbeDiffMatchAndMismatch(t *testing.T) {
	a := ProbeResult{Duration: 10, FormatName: "mov,mp4", VideoCodec: "h264", AudioCodec: "aac"}
	require.Empty(t, probeDiff(a, a))

	b := a
	b.Duration = 10.5
	b.VideoCodec = "hevc"
	d := probeDiff(a, b)
	require.Len(t, d, 2)
	require.Contains(t, d[0], "duration")
	require.Contains(t, d[1], "video codec")

	// Within tolerance is not reported.
	b = a
	b.Duration = 10.05
	require.Empty(t, probeDiff(a, b))
}

func TestDurationDiffReportsDelta(t *testing.T) {
	require.Empty(t, durationDiff(10.0, 10.02))
	d := durationDiff(10.0, 12.5)
	require.Len(t, d, 1)
	require.Contains(t, d[0], "delta")
}

func TestAudioDiffFieldsAndArt(t *testing.T) {
	fast := AudioInfo{Tags: media.Tags{Title: "A", Year: 2020}, HasLyrics: true, Artwork: []byte{1, 2}}
	legacy := AudioInfo{Tags: media.Tags{Title: "B", Year: 2021}, HasLyrics: false, Artwork: nil}

	d := audioDiffer(true)(fast, legacy)
	joined := ""
	for _, s := range d {
		joined += s + "|"
	}
	require.Contains(t, joined, "title")
	require.Contains(t, joined, "year")
	require.Contains(t, joined, "has lyrics")
	require.Contains(t, joined, "cover art bytes")

	// Cover bytes are ignored when art was not requested.
	require.NotContains(t, func() string {
		s := ""
		for _, x := range audioDiffer(false)(fast, AudioInfo{Tags: fast.Tags, HasLyrics: true}) {
			s += x
		}
		return s
	}(), "cover art")

	require.Empty(t, audioDiffer(true)(fast, fast))
}

func TestLyricsDiff(t *testing.T) {
	require.Empty(t, lyricsDiff(LyricsResult{"hi", "lrc"}, LyricsResult{"hi", "lrc"}))
	d := lyricsDiff(LyricsResult{"hi", "lrc"}, LyricsResult{"hello there", "embedded"})
	require.Len(t, d, 2)
}

func TestSubtitleListDiff(t *testing.T) {
	i0, i1 := 0, 1
	a := []utils.SubtitleTrack{{Name: "en", Language: "eng", Codec: "subrip", Index: &i0}}
	require.Empty(t, subtitleListDiff(a, a))

	b := []utils.SubtitleTrack{
		{Name: "en", Language: "eng", Codec: "subrip", Index: &i1},
		{Name: "fr", Language: "fra", Codec: "subrip", Index: &i0},
	}
	d := subtitleListDiff(a, b)
	require.GreaterOrEqual(t, len(d), 2)
	require.Contains(t, d[0], "track count")
}

func TestSubtitleExtractDiff(t *testing.T) {
	require.Empty(t, subtitleExtractDiff("WEBVTT\n", "WEBVTT\n"))
	d := subtitleExtractDiff("WEBVTT\n\n00:00.000 --> 00:01.000\nhi\n", "WEBVTT\n")
	require.Len(t, d, 1)
	require.Contains(t, d[0], "cues")
}

func TestThumbnailDiffIdenticalAndSizeAndLuma(t *testing.T) {
	dark := solidJPEG(t, 64, 48, color.Gray{Y: 20})
	require.Empty(t, thumbnailDiff(dark, dark))

	// Same brightness, different size: only the size is reported.
	bigDark := solidJPEG(t, 128, 96, color.Gray{Y: 20})
	d := thumbnailDiff(dark, bigDark)
	require.Len(t, d, 1)
	require.Contains(t, d[0], "size")

	// Same size, very different brightness: luma is reported.
	bright := solidJPEG(t, 64, 48, color.Gray{Y: 230})
	d = thumbnailDiff(dark, bright)
	require.Len(t, d, 1)
	require.Contains(t, d[0], "mean luma")
}

func TestThumbnailDiffUndecodable(t *testing.T) {
	d := thumbnailDiff([]byte("nope"), solidJPEG(t, 8, 8, color.Gray{Y: 0}))
	require.Len(t, d, 1)
	require.Contains(t, d[0], "decode failed")
}
