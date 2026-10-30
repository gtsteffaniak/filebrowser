package gomedia

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // ffmpeg and go-media thumbnails are JPEG
	_ "image/png"
	"math"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

// Tolerances for shadow comparisons. Differences below these are not reported.
const (
	durationToleranceSec = 0.1  // container duration, seconds
	thumbLumaTolerance   = 8.0  // mean absolute luma difference, 0-255 scale
	thumbSampleGrid      = 64   // samples per axis when comparing thumbnails
	thumbHardLumaDiff    = 60.0 // mean luma difference above this is a real mismatch
	thumbBlackLuma       = 24.0 // mean luma below this counts as a black frame
)

// LyricsResult is the text and format returned by Lyrics.
type LyricsResult struct {
	Text   string
	Format string
}

func probeDiff(fast, legacy ProbeResult) []string {
	var d []string
	if math.Abs(fast.Duration-legacy.Duration) > durationToleranceSec {
		d = append(d, diffNum("duration", fast.Duration, legacy.Duration, "s"))
	}
	if fast.FormatName != legacy.FormatName {
		d = append(d, fmt.Sprintf("format go-media=%q ffmpeg=%q", fast.FormatName, legacy.FormatName))
	}
	if fast.VideoCodec != legacy.VideoCodec {
		d = append(d, fmt.Sprintf("video codec go-media=%q ffmpeg=%q", fast.VideoCodec, legacy.VideoCodec))
	}
	if fast.AudioCodec != legacy.AudioCodec {
		d = append(d, fmt.Sprintf("audio codec go-media=%q ffmpeg=%q", fast.AudioCodec, legacy.AudioCodec))
	}
	return d
}

func durationDiff(fast, legacy float64) []string {
	if math.Abs(fast-legacy) > durationToleranceSec {
		return []string{diffNum("duration", fast, legacy, "s")}
	}
	return nil
}

// audioDiffer compares tags, the lyrics flag, and (when requested) cover bytes.
func audioDiffer(withArt bool) Differ[AudioInfo] {
	return func(fast, legacy AudioInfo) []string {
		var d []string
		str := func(name, a, b string) {
			if a != b {
				d = append(d, fmt.Sprintf("%s go-media=%q ffmpeg=%q", name, a, b))
			}
		}
		str("title", fast.Tags.Title, legacy.Tags.Title)
		str("artist", fast.Tags.Artist, legacy.Tags.Artist)
		str("album", fast.Tags.Album, legacy.Tags.Album)
		str("genre", fast.Tags.Genre, legacy.Tags.Genre)
		if fast.Tags.Year != legacy.Tags.Year {
			d = append(d, fmt.Sprintf("year go-media=%d ffmpeg=%d", fast.Tags.Year, legacy.Tags.Year))
		}
		if fast.Tags.Track != legacy.Tags.Track {
			d = append(d, fmt.Sprintf("track go-media=%d ffmpeg=%d", fast.Tags.Track, legacy.Tags.Track))
		}
		if fast.HasLyrics != legacy.HasLyrics {
			d = append(d, fmt.Sprintf("has lyrics go-media=%t ffmpeg=%t", fast.HasLyrics, legacy.HasLyrics))
		}
		if withArt && len(fast.Artwork) != len(legacy.Artwork) {
			d = append(d, fmt.Sprintf("cover art bytes go-media=%d ffmpeg=%d", len(fast.Artwork), len(legacy.Artwork)))
		}
		return d
	}
}

func lyricsDiff(fast, legacy LyricsResult) []string {
	if fast.Text == legacy.Text && fast.Format == legacy.Format {
		return nil
	}
	var d []string
	if fast.Format != legacy.Format {
		d = append(d, fmt.Sprintf("format go-media=%q ffmpeg=%q", fast.Format, legacy.Format))
	}
	if fast.Text != legacy.Text {
		d = append(d, fmt.Sprintf("text go-media=%d chars ffmpeg=%d chars", len(fast.Text), len(legacy.Text)))
	}
	return d
}

func subtitleListDiff(fast, legacy []utils.SubtitleTrack) []string {
	var d []string
	if len(fast) != len(legacy) {
		d = append(d, fmt.Sprintf("track count go-media=%d ffmpeg=%d", len(fast), len(legacy)))
	}
	for i := 0; i < len(fast) && i < len(legacy); i++ {
		a, b := fast[i], legacy[i]
		if a.Name != b.Name || a.Language != b.Language || a.Codec != b.Codec || a.Title != b.Title {
			d = append(d, fmt.Sprintf("track %d go-media=%q/%q/%q ffmpeg=%q/%q/%q (name/lang/codec)",
				i, a.Name, a.Language, a.Codec, b.Name, b.Language, b.Codec))
		}
		if !sameIndex(a.Index, b.Index) {
			d = append(d, fmt.Sprintf("track %d index go-media=%s ffmpeg=%s", i, indexString(a.Index), indexString(b.Index)))
		}
	}
	return d
}

func sameIndex(a, b *int) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func indexString(p *int) string {
	if p == nil {
		return "nil"
	}
	return fmt.Sprint(*p)
}

func subtitleExtractDiff(fast, legacy string) []string {
	if fast == legacy {
		return nil
	}
	return []string{fmt.Sprintf("WebVTT go-media=%d cues/%d bytes ffmpeg=%d cues/%d bytes",
		strings.Count(fast, "-->"), len(fast), strings.Count(legacy, "-->"), len(legacy))}
}

// thumbnailDiff compares two JPEG thumbnails by size and by mean luma over a
// sampled grid. Pixel-exact comparison is not useful here, because the two
// backends may pick different frames.
//
// Known false positive: when ffmpeg's seek lands on a black intro frame (for
// example ElephantsDream at 10%) while go-media picks a later non-black frame,
// blackMismatch fires even though both backends are behaving reasonably. Shadow
// mode may still log DIFFERS for that case; treat it as expected frame choice
// unless timestamp-matched comparison is added later.
func thumbnailDiff(fast, legacy []byte) []string {
	a, _, errA := image.Decode(bytes.NewReader(fast))
	b, _, errB := image.Decode(bytes.NewReader(legacy))
	if errA != nil || errB != nil {
		return []string{fmt.Sprintf("decode failed go-media=%v ffmpeg=%v", errA, errB)}
	}
	var d []string
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		d = append(d, fmt.Sprintf("size go-media=%dx%d ffmpeg=%dx%d", ab.Dx(), ab.Dy(), bb.Dx(), bb.Dy()))
	}
	mean := meanLumaDiff(a, b)
	if mean > thumbLumaTolerance {
		la, lb := meanLuma(a), meanLuma(b)
		blackMismatch := (la < thumbBlackLuma) != (lb < thumbBlackLuma)
		if mean > thumbHardLumaDiff || blackMismatch {
			d = append(d, fmt.Sprintf("mean luma difference %.1f/255 over %dx%d samples (mean luma go-media=%.1f ffmpeg=%.1f)",
				mean, thumbSampleGrid, thumbSampleGrid, la, lb))
		} else {
			// Both frames are plausible; the backends picked different frames of the video.
			d = append(d, fmt.Sprintf("%smean luma difference %.1f/255 (different frame, both not black)", expectedDiffPrefix, mean))
		}
	}
	return d
}

// meanLuma returns the mean luma (0-255) over the same sampled grid used by meanLumaDiff.
func meanLuma(img image.Image) float64 {
	b := img.Bounds()
	if b.Dx() == 0 || b.Dy() == 0 {
		return 0
	}
	var total float64
	for j := 0; j < thumbSampleGrid; j++ {
		for i := 0; i < thumbSampleGrid; i++ {
			x := b.Min.X + i*b.Dx()/thumbSampleGrid
			y := b.Min.Y + j*b.Dy()/thumbSampleGrid
			total += float64(grayLuma(img.At(x, y)))
		}
	}
	return total / float64(thumbSampleGrid*thumbSampleGrid)
}

// grayLuma returns the luma (Y) of a pixel. The comma-ok form avoids an
// unchecked type assertion; GrayModel.Convert always yields color.Gray, so
// the zero value is only a defensive fallback.
func grayLuma(c color.Color) uint8 {
	g, _ := color.GrayModel.Convert(c).(color.Gray)
	return g.Y
}

// meanLumaDiff samples both images on the same relative grid and returns the
// mean absolute difference of their luma values.
func meanLumaDiff(a, b image.Image) float64 {
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() == 0 || ab.Dy() == 0 || bb.Dx() == 0 || bb.Dy() == 0 {
		return 0
	}
	var total float64
	n := 0
	for j := 0; j < thumbSampleGrid; j++ {
		for i := 0; i < thumbSampleGrid; i++ {
			ax := ab.Min.X + i*ab.Dx()/thumbSampleGrid
			ay := ab.Min.Y + j*ab.Dy()/thumbSampleGrid
			bx := bb.Min.X + i*bb.Dx()/thumbSampleGrid
			by := bb.Min.Y + j*bb.Dy()/thumbSampleGrid
			la := grayLuma(a.At(ax, ay))
			lb := grayLuma(b.At(bx, by))
			total += math.Abs(float64(la) - float64(lb))
			n++
		}
	}
	return total / float64(n)
}
