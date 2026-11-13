package gomedia

import (
	"context"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ffprobeKeyframes runs the same packet probe as go-ffmpeg's
// ProbeVideoKeyframeTimes: key packets of v:0, pts_time falling back to
// dts_time, sorted and de-duplicated within 1 ms.
func ffprobeKeyframes(t *testing.T, path string) []float64 {
	t.Helper()
	requireTool(t, "ffprobe")
	out, err := exec.Command("ffprobe", "-v", "error", "-select_streams", "v:0",
		"-show_entries", "packet=pts_time,dts_time,flags", "-of", "csv=p=0", path).Output()
	require.NoError(t, err)
	var times []float64
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Split(strings.TrimSpace(line), ",")
		if len(f) < 3 || !strings.Contains(f[2], "K") {
			continue
		}
		v, perr := strconv.ParseFloat(f[0], 64)
		if perr != nil {
			v, perr = strconv.ParseFloat(f[1], 64)
		}
		if perr == nil {
			times = append(times, v)
		}
	}
	sort.Float64s(times)
	return dedupeKeyframeTimes(times)
}

// encodeKeyframeClip makes a short clip with a fixed GOP. B-frames give MP4
// files a ctts table and an edit list, the hard case for the timestamp model.
func encodeKeyframeClip(t *testing.T, name string, args ...string) string {
	t.Helper()
	requireTool(t, "ffmpeg")
	path := filepath.Join(t.TempDir(), name)
	full := []string{"-v", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=160x120:rate=25:duration=6"}
	full = append(full, args...)
	full = append(full, path)
	if out, err := exec.Command("ffmpeg", full...).CombinedOutput(); err != nil {
		t.Skipf("ffmpeg cannot encode %s: %v: %s", name, err, out)
	}
	return path
}

// requireSameKeyframes fails unless got matches want within the shadow
// tolerance.
func requireSameKeyframes(t *testing.T, want, got []float64) {
	t.Helper()
	require.Equal(t, len(want), len(got), "keyframe count: ffprobe=%v go-media=%v", want, got)
	for i := range want {
		require.InDelta(t, want[i], got[i], keyframeShadowTolerance, "keyframe %d: ffprobe=%v go-media=%v", i, want, got)
	}
}

// TestKeyframeTimesMatchFFprobe compares go-media keyframe indexes with
// ffprobe for fixtures and encoded clips, including B-frame MP4/MOV.
func TestKeyframeTimesMatchFFprobe(t *testing.T) {
	if testing.Short() {
		t.Skip("encodes clips with ffmpeg")
	}
	withGate(t, false)
	x264 := []string{"-c:v", "libx264", "-g", "50", "-keyint_min", "50", "-sc_threshold", "0", "-bf", "2", "-pix_fmt", "yuv420p"}
	cases := map[string]string{
		"fixture.mp4":  fixtureMP4,
		"fixture.webm": fixtureWebM,
		"bframes.mp4":  encodeKeyframeClip(t, "bframes.mp4", x264...),
		"bframes.mov":  encodeKeyframeClip(t, "bframes.mov", x264...),
		"bframes.mkv":  encodeKeyframeClip(t, "bframes.mkv", x264...),
		"mpeg4.m4v":    encodeKeyframeClip(t, "mpeg4.m4v", "-c:v", "mpeg4", "-g", "30", "-f", "mp4"),
	}
	for name, path := range cases {
		t.Run(name, func(t *testing.T) {
			want := ffprobeKeyframes(t, path)
			got, ok := KeyframeTimes(context.Background(), path, nil)
			require.True(t, ok)
			requireSameKeyframes(t, want, got)

			within, ok := KeyframeTimesWithin(context.Background(), path, 2.5)
			require.True(t, ok)
			require.NotEmpty(t, within)
			require.LessOrEqual(t, len(within), len(got))
			for i := range within {
				require.Equal(t, got[i], within[i])
				require.LessOrEqual(t, within[i]-got[0], 2.5+1e-9)
			}
		})
	}
}

// TestKeyframeTimesFallsBack checks the cases that must leave the index to
// ffprobe.
func TestKeyframeTimesFallsBack(t *testing.T) {
	withGate(t, false)
	dir := t.TempDir()

	// Not an indexed container: no file open, ffprobe answers.
	ts := filepath.Join(dir, "clip.ts")
	require.NoError(t, os.WriteFile(ts, []byte{0x47}, 0o644))
	_, ok := KeyframeTimes(context.Background(), ts, nil)
	require.False(t, ok)

	// Indexed extension but not a real container.
	junk := filepath.Join(dir, "junk.mp4")
	require.NoError(t, os.WriteFile(junk, []byte("not a video"), 0o644))
	_, ok = KeyframeTimes(context.Background(), junk, nil)
	require.False(t, ok)

	// Missing file.
	_, ok = KeyframeTimes(context.Background(), filepath.Join(dir, "missing.mp4"), nil)
	require.False(t, ok)

	// Cancelled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, ok = KeyframeTimes(ctx, fixtureMP4, nil)
	require.False(t, ok)
}

// TestKeyframeTimesGate checks that the Always use ffmpeg setting disables
// the go-media keyframe index.
func TestKeyframeTimesGate(t *testing.T) {
	withGate(t, true)
	_, ok := KeyframeTimes(context.Background(), fixtureMP4, nil)
	require.False(t, ok, "KeyframeTimes must skip go-media when the gate is on")
	_, ok = KeyframeTimesWithin(context.Background(), fixtureMP4, 10)
	require.False(t, ok)
}

// TestDedupeKeyframeTimes checks sorting, filtering, and the 1 ms dedupe.
func TestDedupeKeyframeTimes(t *testing.T) {
	got := dedupeKeyframeTimes([]float64{2, -1, 0, 0.0005, 1, 1.001, 1.0021, math.NaN(), math.Inf(1)})
	require.Equal(t, []float64{0, 1, 1.0021, 2}, got)
	require.Empty(t, dedupeKeyframeTimes(nil))
}

// TestCtxReaderAt checks that reads stop once the context is cancelled.
func TestCtxReaderAt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := ctxReaderAt{ctx: ctx, r: strings.NewReader("abcdef")}
	buf := make([]byte, 3)
	n, err := r.ReadAt(buf, 1)
	require.NoError(t, err)
	require.Equal(t, "bcd", string(buf[:n]))
	cancel()
	_, err = r.ReadAt(buf, 0)
	require.ErrorIs(t, err, context.Canceled)
}

// TestKeyframeDiff checks the shadow comparison: a subset is expected, an
// extra keyframe is a real difference.
func TestKeyframeDiff(t *testing.T) {
	require.Empty(t, keyframeDiff([]float64{0, 2, 4}, []float64{0, 2.001, 4}))

	subset := keyframeDiff([]float64{0, 4}, []float64{0, 2, 4, 6})
	require.Len(t, subset, 1)
	require.True(t, strings.HasPrefix(subset[0], expectedDiffPrefix), subset[0])
	require.Contains(t, subset[0], "2 of 4")

	extra := keyframeDiff([]float64{0, 1, 2}, []float64{0, 2})
	require.Len(t, extra, 1)
	require.False(t, strings.HasPrefix(extra[0], expectedDiffPrefix), extra[0])
}
