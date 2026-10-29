package gomedia

import (
	"bytes"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

// Fixtures shipped with the Playwright suite. Paths are relative to this package.
const (
	fixtureDir  = "../../../frontend/tests/playwright-files/myfolder/testdata"
	fixtureMP4  = fixtureDir + "/sample-640x480.mp4"
	fixtureWebM = fixtureDir + "/clip-a.webm"
)

// withGate sets the admin "Always use ffmpeg" setting for one test.
func withGate(t *testing.T, alwaysFfmpeg bool) {
	t.Helper()
	prev := settings.CurrentMediaSettings()
	cur := prev
	cur.AlwaysUseFfmpeg = alwaysFfmpeg
	settings.ApplyMediaSettings(cur)
	t.Cleanup(func() { settings.ApplyMediaSettings(prev) })
}

// requireTool skips the test when an ffmpeg binary is not on PATH.
func requireTool(t *testing.T, name string) {
	t.Helper()
	if _, err := exec.LookPath(name); err != nil {
		t.Skipf("%s not installed", name)
	}
}

// ffprobeValue returns one ffprobe value for path. args select the field.
func ffprobeValue(t *testing.T, path string, args ...string) string {
	t.Helper()
	requireTool(t, "ffprobe")
	full := append([]string{"-v", "error"}, args...)
	full = append(full, "-of", "default=nw=1:nk=1", path)
	out, err := exec.Command("ffprobe", full...).Output()
	require.NoError(t, err)
	return strings.TrimSpace(string(out))
}

// ffprobeMedia returns the ffprobe duration, format name, and first video and
// audio codec names. One ffprobe call per field keeps the parsing simple.
func ffprobeMedia(t *testing.T, path string) (dur float64, format, video, audio string) {
	t.Helper()
	dur, _ = strconv.ParseFloat(ffprobeValue(t, path, "-show_entries", "format=duration"), 64)
	format = ffprobeValue(t, path, "-show_entries", "format=format_name")
	video = ffprobeValue(t, path, "-select_streams", "v:0", "-show_entries", "stream=codec_name")
	audio = ffprobeValue(t, path, "-select_streams", "a:0", "-show_entries", "stream=codec_name")
	return dur, format, video, audio
}

// makeMP3 generates a 2s MP3 with an ID3 title tag. The repo's MP3 fixture is
// empty, so ffmpeg makes a real one.
func makeMP3(t *testing.T) string {
	t.Helper()
	requireTool(t, "ffmpeg")
	path := filepath.Join(t.TempDir(), "tone.mp3")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-metadata", "title=Gomedia Title",
		"-c:a", "libmp3lame", path).CombinedOutput()
	require.NoError(t, err, string(out))
	return path
}

func TestEnabledFollowsAdminGate(t *testing.T) {
	withGate(t, true)
	require.False(t, Enabled(), "admin gate on must disable go-media")

	withGate(t, false)
	require.True(t, Enabled(), "default must keep go-media on")
}

func TestGateOffSkipsEveryPath(t *testing.T) {
	withGate(t, true)
	mp3 := makeMP3(t)

	_, ok := Probe(fixtureMP4, nil)
	require.False(t, ok, "Probe must skip go-media when the gate is on")

	_, ok = Duration(fixtureMP4, nil)
	require.False(t, ok, "Duration must skip go-media when the gate is on")

	_, ok = Audio(mp3, true, nil)
	require.False(t, ok, "Audio must skip go-media when the gate is on")

	_, _, ok = Lyrics(mp3, nil)
	require.False(t, ok, "Lyrics must skip go-media when the gate is on")

	_, ok = VideoThumbnail(fixtureMP4, 256, 256, 72, 10, nil)
	require.False(t, ok, "VideoThumbnail must skip go-media when the gate is on")

	_, ok = EmbeddedSubtitles(fixtureMP4, time.Now(), nil)
	require.False(t, ok, "EmbeddedSubtitles must skip go-media when the gate is on")

	_, ok = ExtractSubtitle(fixtureMP4, 0, nil)
	require.False(t, ok, "ExtractSubtitle must skip go-media when the gate is on")
}

func TestProbeMP4MatchesFFprobe(t *testing.T) {
	withGate(t, false)
	wantDur, wantFormat, wantVideo, wantAudio := ffprobeMedia(t, fixtureMP4)

	got, ok := Probe(fixtureMP4, nil)
	require.True(t, ok)
	require.InDelta(t, wantDur, got.Duration, 0.1)
	require.Equal(t, wantFormat, got.FormatName)
	require.Equal(t, wantVideo, got.VideoCodec)
	require.Equal(t, wantAudio, got.AudioCodec)
}

func TestProbeWebMMatchesFFprobe(t *testing.T) {
	withGate(t, false)
	wantDur, _, wantVideo, wantAudio := ffprobeMedia(t, fixtureWebM)

	got, ok := Probe(fixtureWebM, nil)
	if !ok {
		t.Skip("go-media does not handle this WebM file; ffmpeg fallback covers it")
	}
	// ffprobe reports "matroska,webm" for WebM; go-media's normalized name must match it.
	require.Equal(t, "matroska,webm", got.FormatName)
	require.InDelta(t, wantDur, got.Duration, 0.1)
	require.Equal(t, wantVideo, got.VideoCodec)
	require.Equal(t, wantAudio, got.AudioCodec)
}

func TestDurationMP3MatchesFFprobe(t *testing.T) {
	withGate(t, false)
	mp3 := makeMP3(t)
	wantDur, _, _, _ := ffprobeMedia(t, mp3)

	got, ok := Duration(mp3, nil)
	if !ok {
		t.Skip("go-media returned no duration for this MP3; ffmpeg fallback covers it")
	}
	// MP3 durations may be estimates; allow a looser tolerance.
	require.InDelta(t, wantDur, got, 0.5)
}

func TestAudioTagsFromMP3(t *testing.T) {
	withGate(t, false)
	mp3 := makeMP3(t)

	info, ok := Audio(mp3, true, nil)
	require.True(t, ok)
	require.Equal(t, "Gomedia Title", info.Tags.Title)
	require.False(t, info.HasLyrics)
	require.Empty(t, info.Artwork, "generated MP3 has no cover art")
}

func TestVideoThumbnailFitsBoxAndKeepsAspect(t *testing.T) {
	withGate(t, false)

	data, ok := VideoThumbnail(fixtureMP4, 256, 256, 72, 10, nil)
	require.True(t, ok)
	img, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	b := img.Bounds()
	require.LessOrEqual(t, b.Dx(), 256)
	require.LessOrEqual(t, b.Dy(), 256)
	// 640x480 source is 4:3, so a 256 box gives 256x192. Allow 1px rounding.
	require.InDelta(t, 256, b.Dx(), 1)
	require.InDelta(t, 192, b.Dy(), 1)
}

func TestVideoThumbnailOriginalSizeIsNotScaled(t *testing.T) {
	withGate(t, false)

	// Zero bounds mean "original": the output must match the source size exactly.
	data, ok := VideoThumbnail(fixtureMP4, 0, 0, 90, 10, nil)
	require.True(t, ok)
	img, err := jpeg.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	require.Equal(t, 640, img.Bounds().Dx())
	require.Equal(t, 480, img.Bounds().Dy())
}

func TestSubtitleListSkipsUnsupportedContainer(t *testing.T) {
	withGate(t, false)

	// MP3 has no subtitle streams in go-media. Report unsupported so the caller
	// uses ffmpeg. Do not return an empty list, which would hide real subtitles.
	_, ok := EmbeddedSubtitles(makeMP3(t), time.Now(), nil)
	require.False(t, ok)
}

func TestGarbageFileFallsBack(t *testing.T) {
	withGate(t, false)

	path := filepath.Join(t.TempDir(), "not-media.mp4")
	require.NoError(t, os.WriteFile(path, bytes.Repeat([]byte("x"), 4096), 0o600))

	_, ok := Probe(path, nil)
	require.False(t, ok)
	_, ok = Duration(path, nil)
	require.False(t, ok)
	_, ok = VideoThumbnail(path, 256, 256, 72, 10, nil)
	require.False(t, ok)
}

// TestEmbeddedSubtitleIndexMatchesFFprobe builds an MKV with a known subtitle
// track and checks that go-media's index is the one ffmpeg uses. A wrong index
// would extract the wrong track.
func TestEmbeddedSubtitleIndexMatchesFFprobe(t *testing.T) {
	withGate(t, false)
	requireTool(t, "ffmpeg")

	dir := t.TempDir()
	srt := filepath.Join(dir, "in.srt")
	require.NoError(t, os.WriteFile(srt, []byte("1\n00:00:00,000 --> 00:00:01,000\nHello gomedia\n"), 0o600))
	mkv := filepath.Join(dir, "subs.mkv")
	out, err := exec.Command("ffmpeg", "-v", "error", "-y",
		"-f", "lavfi", "-i", "color=c=black:s=64x64:d=1",
		"-i", srt,
		"-map", "0:v", "-map", "1:s",
		"-c:v", "mpeg4", "-c:s", "srt", mkv).CombinedOutput()
	require.NoError(t, err, string(out))

	tracks, ok := EmbeddedSubtitles(mkv, time.Now(), nil)
	require.True(t, ok, "go-media must list MKV subtitle tracks")
	require.Len(t, tracks, 1)
	require.NotNil(t, tracks[0].Index)

	// ffprobe's subtitle stream index is the ground truth.
	ffIndex := ffprobeValue(t, mkv, "-select_streams", "s:0", "-show_entries", "stream=index")
	require.Equal(t, ffIndex, strconv.Itoa(*tracks[0].Index))

	vtt, ok := ExtractSubtitle(mkv, *tracks[0].Index, nil)
	require.True(t, ok)
	require.True(t, strings.HasPrefix(strings.TrimSpace(vtt), "WEBVTT"), "output must be WebVTT")
	require.Contains(t, vtt, "Hello gomedia")
}
