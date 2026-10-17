package transcoding

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-ffmpeg/capabilities"
)

func withTranscodeProfiles(t *testing.T) {
	t.Helper()
	original := settings.Config.Integrations.Media.Transcode.Profiles
	t.Cleanup(func() { settings.Config.Integrations.Media.Transcode.Profiles = original })
	settings.Config.Integrations.Media.Transcode.Profiles = settings.DefaultTranscodeProfiles()
}

// A profile that caps the height above the source resolution must not select a
// higher bitrate rung: the scale filter can only cap, never upscale.
func TestEncodeProfileForModeClampsToSourceHeight(t *testing.T) {
	withTranscodeProfiles(t)
	small := ffmpeg.StreamInfo{Height: 270, VideoBitrate: 0}

	if got := bitrateKbps(encodeProfileForMode(small, ProfileQuality).Bitrate.Target); got != 1500 {
		t.Fatalf("quality bitrate for 270p source = %d kbps, want the 1500 rung (no upscale)", got)
	}
	if got := bitrateKbps(encodeProfileForMode(small, ProfileBalanced).Bitrate.Target); got != 4000 {
		t.Fatalf("balanced bitrate = %d kbps, want the 4000 cap", got)
	}
	if got := bitrateKbps(encodeProfileForMode(small, ProfileDataSaver).Bitrate.Target); got != 1500 {
		t.Fatalf("data saver bitrate = %d kbps, want the 1500 cap", got)
	}
}

// Bitrate caps are ceilings, not targets: -b:v is a target average, so a source
// already below the cap must keep its own bitrate instead of being inflated.
func TestEncodeProfileForModeFloorsBitrateAtSource(t *testing.T) {
	withTranscodeProfiles(t)
	small := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 800_000}
	for _, profile := range []Profile{ProfileQuality, ProfileBalanced, ProfileDataSaver} {
		if kbps := bitrateKbps(encodeProfileForMode(small, profile).Bitrate.Target); kbps != 800 {
			t.Fatalf("%s bitrate for an 800 kbps source = %d kbps, want 800", profile, kbps)
		}
	}
}

func TestEncodeProfileForModeKeepsSourceBitrate(t *testing.T) {
	withTranscodeProfiles(t)
	// 720p H.264 at ~2500 kbps sits below the balanced cap, so the source
	// bitrate wins.
	src := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 2_500_000}
	got := encodeProfileForMode(src, ProfileBalanced)
	if kbps := bitrateKbps(got.Bitrate.Target); kbps != 2500 {
		t.Fatalf("bitrate = %d kbps, want the source 2500 kbps", kbps)
	}
}

func TestProfileForcesVideoTranscode(t *testing.T) {
	if profileForcesVideoTranscode(ProfileQuality) {
		t.Fatal("quality must keep the source when stream copy is possible")
	}
	for _, profile := range []Profile{ProfileBalanced, ProfileDataSaver} {
		if !profileForcesVideoTranscode(profile) {
			t.Fatalf("%s must always transcode video", profile)
		}
	}
}

func TestFMP4CopyableVideoCodec(t *testing.T) {
	for _, codec := range []string{"h264", "avc", "avc1"} {
		if !fmp4CopyableVideoCodec(codec) {
			t.Fatalf("%s should be fMP4-copyable", codec)
		}
	}
	// Only h264 is universal under MSE: VP9 has no Safari support and HEVC is
	// inconsistent outside Apple hardware, so both transcode instead.
	for _, codec := range []string{"hevc", "h265", "hvc1", "hev1", "vp9", "vp09.00.40.08", "vp8", "av1", "wmv3", "mpeg4", "theora"} {
		if fmp4CopyableVideoCodec(codec) {
			t.Fatalf("%s must transcode for broad hls.js compatibility", codec)
		}
	}
}

func TestFMP4CopyEligibility(t *testing.T) {
	tests := []struct {
		name          string
		info          ffmpeg.StreamInfo
		mustTranscode bool
		wantRemux     bool
		wantVideoCopy bool
	}{
		{"h264+aac remuxes", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"}, false, true, false},
		{"h264+ac3 copies video only", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "ac3"}, false, false, true},
		{"h264 without audio remuxes", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264"}, false, true, false},
		{"hevc transcodes for compatibility", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "hevc", AudioCodec: "aac"}, false, false, false},
		{"vp9 transcodes for compatibility", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp9", AudioCodec: "opus"}, false, false, false},
		{"vp8 never copies", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "vp8", AudioCodec: "aac"}, false, false, false},
		{"av1 never copies", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "av1", AudioCodec: "aac"}, false, false, false},
		{"forced transcode copies nothing", ffmpeg.StreamInfo{HasVideo: true, VideoCodec: "h264", AudioCodec: "aac"}, true, false, false},
		{"no video copies nothing", ffmpeg.StreamInfo{AudioCodec: "aac"}, false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			remux, videoCopy := fmp4CopyEligibility(tc.info, tc.mustTranscode)
			if remux != tc.wantRemux || videoCopy != tc.wantVideoCopy {
				t.Fatalf("got (remux=%v, videoCopy=%v), want (%v, %v)", remux, videoCopy, tc.wantRemux, tc.wantVideoCopy)
			}
		})
	}
}

func TestProfileRequiresBitrateTranscode(t *testing.T) {
	withTranscodeProfiles(t)

	// Data saver pins 1500 kbps: a higher-bitrate or unknown-bitrate source must
	// be re-encoded, since stream copy ignores the cap.
	high := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 4_000_000}
	if !profileRequiresBitrateTranscode(ProfileDataSaver, high) {
		t.Fatal("a source above the cap must force a transcode")
	}
	unknown := ffmpeg.StreamInfo{Height: 480, VideoBitrate: 0}
	if !profileRequiresBitrateTranscode(ProfileDataSaver, unknown) {
		t.Fatal("an unknown source bitrate must force a transcode to honor the cap")
	}
	low := ffmpeg.StreamInfo{Height: 720, VideoBitrate: 1_000_000}
	if profileRequiresBitrateTranscode(ProfileDataSaver, low) {
		t.Fatal("a source below the cap does not need a re-encode")
	}
	// Quality pins no bitrate, so it never forces a transcode for the bitrate's sake.
	if profileRequiresBitrateTranscode(ProfileQuality, high) {
		t.Fatal("quality has no bitrate cap and must not force a transcode")
	}
}

func bitrateKbps(target string) int {
	trimmed := strings.TrimSuffix(target, "k")
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return -1
	}
	return n
}

func TestContinuousSeekParams(t *testing.T) {
	t.Parallel()
	const seg = 4.0

	tests := []struct {
		name       string
		startSec   float64
		wantIndex  int
		wantFFmpeg float64
	}{
		{name: "zero", startSec: 0, wantIndex: 0, wantFFmpeg: 0},
		{name: "within first segment", startSec: 3.44, wantIndex: 0, wantFFmpeg: 0},
		{name: "segment boundary", startSec: 4, wantIndex: 1, wantFFmpeg: 4},
		{name: "mid segment", startSec: 5.5, wantIndex: 1, wantFFmpeg: 4},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			idx, ffStart := continuousSeekParams(tc.startSec, seg)
			if idx != tc.wantIndex || ffStart != tc.wantFFmpeg {
				t.Fatalf("continuousSeekParams(%v, %v) = (%d, %v), want (%d, %v)",
					tc.startSec, seg, idx, ffStart, tc.wantIndex, tc.wantFFmpeg)
			}
		})
	}
}

func TestTranscodePlanUsesFixedSegmentGrid(t *testing.T) {
	t.Parallel()
	const segDur = 4.0
	duration := 30.5

	// Sparse WMV-like keyframes that do not match the output grid.
	keyframes := []float64{0, 3.909, 7.818, 11.727}
	keyframes = ffmpeg.SanitizeHLSKeyframes(keyframes, duration)
	_, keyframeDurs := ffmpeg.BuildHLSSegmentTimeline(duration, keyframes, segDur, 0)

	_, fixedDurs := ffmpeg.BuildHLSSegmentTimeline(duration, nil, segDur, 0)

	if len(keyframeDurs) == len(fixedDurs) {
		same := true
		for i := range fixedDurs {
			if keyframeDurs[i] != fixedDurs[i] {
				same = false
				break
			}
		}
		if same {
			t.Fatal("expected keyframe-based durations to differ from fixed grid for sparse keyframes")
		}
	}

	lastFixed := fixedDurs[len(fixedDurs)-1]
	if lastFixed <= 0 || lastFixed > segDur {
		t.Fatalf("tail segment duration %v out of range", lastFixed)
	}
	for i, dur := range fixedDurs[:len(fixedDurs)-1] {
		if dur != segDur {
			t.Fatalf("fixed segment %d duration = %v, want %v", i, dur, segDur)
		}
	}
}

// Containers with a non-zero format start_time (e.g. some MKV/MP4 muxes) carry
// keyframe times on a shifted container timeline. Segment planning must
// rebase them onto the 0-based media timeline the HLS output covers.
func TestTranscodePlanNormalizesContainerStartTime(t *testing.T) {
	t.Parallel()
	const segDur = 4.0
	const startTime = 1.4

	// Same media as the zero-offset case: 30.5s of content whose format
	// duration includes the 1.4s pre-roll offset.
	containerDuration := 30.5 + startTime
	containerKeyframes := []float64{startTime, startTime + 3.909, startTime + 7.818, startTime + 11.727}

	sanitized, mediaDuration := ffmpeg.PrepareHLSMediaTimeline(containerKeyframes, containerDuration, startTime)
	if mediaDuration < 30.4 || mediaDuration > 30.6 {
		t.Fatalf("media duration = %v, want ~30.5", mediaDuration)
	}
	_, offsetDurs := ffmpeg.BuildHLSSegmentTimeline(mediaDuration, sanitized, segDur, 0)

	keyframes := ffmpeg.SanitizeHLSKeyframes([]float64{0, 3.909, 7.818, 11.727}, 30.5)
	_, zeroOffsetDurs := ffmpeg.BuildHLSSegmentTimeline(30.5, keyframes, segDur, 0)

	if len(offsetDurs) != len(zeroOffsetDurs) {
		t.Fatalf("normalized timeline has %d segments, want %d", len(offsetDurs), len(zeroOffsetDurs))
	}
	for i := range zeroOffsetDurs {
		if offsetDurs[i] != zeroOffsetDurs[i] {
			t.Fatalf("segment %d duration = %v, want %v", i, offsetDurs[i], zeroOffsetDurs[i])
		}
	}

	// The lib also normalizes internally when startTimeSec is passed directly.
	_, libDurs := ffmpeg.BuildHLSSegmentTimeline(containerDuration, containerKeyframes, segDur, startTime)
	if len(libDurs) != len(zeroOffsetDurs) {
		t.Fatalf("lib-normalized timeline has %d segments, want %d", len(libDurs), len(zeroOffsetDurs))
	}
}

// The encoder backend preference resolves profile override, then the
// transcode-level default; hardware backends are capability-guarded so an
// unavailable one degrades to auto instead of erroring the job.
func TestPreferredEncoderAccel(t *testing.T) {
	withTranscodeProfiles(t)
	origBackend := settings.Config.Integrations.Media.Transcode.EncoderBackend
	t.Cleanup(func() { settings.Config.Integrations.Media.Transcode.EncoderBackend = origBackend })

	settings.Config.Integrations.Media.Transcode.EncoderBackend = ""
	if got := preferredEncoderAccel(ProfileBalanced); got != "" {
		t.Fatalf("auto backend = %q, want \"\"", got)
	}

	settings.Config.Integrations.Media.Transcode.EncoderBackend = "cpu"
	if got := preferredEncoderAccel(ProfileBalanced); got != capabilities.AccelNone {
		t.Fatalf("cpu backend = %q, want %q", got, capabilities.AccelNone)
	}
	// A per-preset override beats the transcode-level default.
	profiles := settings.DefaultTranscodeProfiles()
	profiles.DataSaver.EncoderBackend = "cpu"
	settings.Config.Integrations.Media.Transcode.Profiles = profiles
	if got := preferredEncoderAccel(ProfileDataSaver); got != capabilities.AccelNone {
		t.Fatalf("preset-override backend = %q, want %q", got, capabilities.AccelNone)
	}

	// No ffmpeg service in tests -> no capabilities -> hardware backends must
	// degrade to auto rather than resolving (ResolveEncoder would error).
	settings.Config.Integrations.Media.Transcode.EncoderBackend = "nvenc"
	if got := preferredEncoderAccel(ProfileBalanced); got != "" {
		t.Fatalf("unavailable nvenc = %q, want auto \"\"", got)
	}
}
